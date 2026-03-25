package v1

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/sampleinstance"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
)

// ActuatorService implements the Connect RPC interface for ActuatorService.
type ActuatorService struct {
	v1connect.UnimplementedActuatorServiceHandler
	store                 *store.Store
	profile               *config.Profile
	schemaSyncer          *schemasync.Syncer
	sampleInstanceManager *sampleinstance.Manager
}

// NewActuatorService creates a new ActuatorService.
func NewActuatorService(
	store *store.Store,
	profile *config.Profile,
	schemaSyncer *schemasync.Syncer,
	sampleInstanceManager *sampleinstance.Manager,
) *ActuatorService {
	return &ActuatorService{
		store:                 store,
		profile:               profile,
		schemaSyncer:          schemaSyncer,
		sampleInstanceManager: sampleInstanceManager,
	}
}

// GetActuatorInfo gets the actuator info.
func (s *ActuatorService) GetActuatorInfo(
	ctx context.Context,
	_ *connect.Request[v1pb.GetActuatorInfoRequest],
) (*connect.Response[v1pb.ActuatorInfo], error) {
	var workspaceID string
	if !s.profile.SaaS {
		ws, err := s.store.GetWorkspaceID(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		workspaceID = ws
	}
	info, err := s.getServerInfo(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(info), nil
}

// GetWorkspaceActuatorInfo gets workspace-scoped actuator info. Requires authentication.
// Workspace validation is handled by the ACL layer (resource_reference on name field).
func (s *ActuatorService) GetWorkspaceActuatorInfo(
	ctx context.Context,
	_ *connect.Request[v1pb.GetWorkspaceActuatorInfoRequest],
) (*connect.Response[v1pb.ActuatorInfo], error) {
	info, err := s.getServerInfo(ctx, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(info), nil
}

// DeleteCache deletes the cache.
func (s *ActuatorService) DeleteCache(
	_ context.Context,
	_ *connect.Request[v1pb.DeleteCacheRequest],
) (*connect.Response[emptypb.Empty], error) {
	s.store.DeleteCache()
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GetResourcePackage gets the theme resources.
// Serves both /v1/actuator/resources and /v1/{name=workspaces/*}/actuator/resources.
func (s *ActuatorService) GetResourcePackage(
	ctx context.Context,
	req *connect.Request[v1pb.GetResourcePackageRequest],
) (*connect.Response[v1pb.ResourcePackage], error) {
	var workspaceID string
	if !s.profile.SaaS {
		ws, err := s.store.GetWorkspaceID(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		workspaceID = ws
	} else if req.Msg.Name == "" {
		reqWorkspaceID, err := common.GetWorkspaceID(req.Msg.Name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = reqWorkspaceID
	}

	pkg := &v1pb.ResourcePackage{}

	if workspaceID != "" {
		workspaceProfileSetting, err := s.store.GetWorkspaceProfileSetting(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find workspace profile setting"))
		}
		pkg.Logo = []byte(workspaceProfileSetting.BrandingLogo)
	}

	return connect.NewResponse(pkg), nil
}

// SetupSample sets up the sample project and instance.
func (s *ActuatorService) SetupSample(
	ctx context.Context,
	_ *connect.Request[v1pb.SetupSampleRequest],
) (*connect.Response[emptypb.Empty], error) {
	if s.profile.SaaS {
		// skip sample setup in SaaS
		slog.Debug("sample is not available for SaaS")
		return connect.NewResponse(&emptypb.Empty{}), nil
	}
	user, ok := GetUserFromContext(ctx)
	if !ok || user == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	if s.sampleInstanceManager != nil {
		if err := s.sampleInstanceManager.GenerateOnboardingData(ctx, common.GetWorkspaceIDFromContext(ctx), user, s.schemaSyncer); err != nil {
			// When running inside docker on mac, we sometimes get database does not exist error.
			// This is due to the docker overlay storage incompatibility with mac OS file system.
			// Onboarding error is not critical, so we just emit an error log.
			slog.Error("failed to prepare onboarding data", log.BBError(err))
		}
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *ActuatorService) getServerInfo(ctx context.Context, workspaceID string) (*v1pb.ActuatorInfo, error) {
	serverInfo := v1pb.ActuatorInfo{
		Version:             s.profile.Version,
		GitCommit:           s.profile.GitCommit,
		Saas:                s.profile.SaaS,
		Demo:                s.profile.Demo,
		LastActiveTime:      timestamppb.New(time.Unix(s.profile.LastActiveTS.Load(), 0)),
		Docker:              s.profile.IsDocker,
		ExternalUrlFromFlag: s.profile.ExternalURL != "",
		ReplicaCount:        1,
		Restriction: &v1pb.Restriction{
			PasswordRestriction: &v1pb.WorkspaceProfileSetting_PasswordRestriction{
				MinLength: 8,
			},
		},
		NeedAdminSetup: true,
	}

	if workspaceID != "" {
		serverInfo.Workspace = common.FormatWorkspace(workspaceID)

		defaultProjectID, err := s.store.GetDefaultProjectID(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get default project"))
		}
		serverInfo.DefaultProject = common.FormatProject(defaultProjectID)

		activeEndUserCount, err := s.store.CountActiveEndUsersPerWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		serverInfo.NeedAdminSetup = activeEndUserCount == 0
		serverInfo.ActivatedUserCount = int32(activeEndUserCount)

		setting, err := s.store.GetWorkspaceProfileSetting(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find workspace setting"))
		}
		serverInfo.Restriction = &v1pb.Restriction{
			PasswordRestriction:    convertPasswordRestriction(setting.GetPasswordRestriction()),
			DisallowSignup:         setting.DisallowSignup,
			DisallowPasswordSignin: setting.DisallowPasswordSignin,
		}

		// Check if sample instances are available
		hasSampleInstances, _ := s.store.HasSampleInstances(ctx, workspaceID)
		serverInfo.EnableSample = hasSampleInstances

		// Prefer command-line flag over database value for external URL
		externalURL := setting.ExternalUrl
		if s.profile.ExternalURL != "" {
			externalURL = s.profile.ExternalURL
		}
		serverInfo.ExternalUrl = externalURL

		activatedInstanceCount, err := s.store.GetActivatedInstanceCount(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to count activated instance"))
		}
		serverInfo.ActivatedInstanceCount = int32(activatedInstanceCount)

		activeInstanceCount, err := s.store.CountActiveInstances(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to count total instance"))
		}
		serverInfo.TotalInstanceCount = int32(activeInstanceCount)
	}

	return &serverInfo, nil
}

// convertPasswordRestriction converts store PasswordRestriction to v1 PasswordRestriction.
func convertPasswordRestriction(pr *storepb.WorkspaceProfileSetting_PasswordRestriction) *v1pb.WorkspaceProfileSetting_PasswordRestriction {
	if pr == nil {
		return nil
	}
	return &v1pb.WorkspaceProfileSetting_PasswordRestriction{
		MinLength:                         pr.MinLength,
		RequireNumber:                     pr.RequireNumber,
		RequireLetter:                     pr.RequireLetter,
		RequireUppercaseLetter:            pr.RequireUppercaseLetter,
		RequireSpecialCharacter:           pr.RequireSpecialCharacter,
		RequireResetPasswordForFirstLogin: pr.RequireResetPasswordForFirstLogin,
		PasswordRotation:                  pr.PasswordRotation,
	}
}
