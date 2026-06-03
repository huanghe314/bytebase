// Package enterprise provides license service.
package enterprise

import (
	"context"
	"math"

	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

// LicenseParams holds the parameters for creating license claims.
type LicenseParams struct {
	Plan        string
	Seats       int
	Instances   int
	WorkspaceID string
}

// licenseClaims holds the parsed license claims.
type licenseClaims struct {
	Plan            string
	Seats           int
	Instances       int
	ActiveInstances int
	WorkspaceID     string
}

// newLicenseClaims creates license claims from params.
// In a no-license-required build, instances and activeInstances are always equal.
func newLicenseClaims(params *LicenseParams) *licenseClaims {
	return &licenseClaims{
		Plan:            params.Plan,
		Seats:           params.Seats,
		Instances:       params.Instances,
		ActiveInstances: params.Instances,
		WorkspaceID:     params.WorkspaceID,
	}
}

// LicenseService provides enterprise license checks.
// All methods return unrestricted access since no license is required.
type LicenseService struct{}

// NewLicenseService creates a new LicenseService.
func NewLicenseService() *LicenseService {
	return &LicenseService{}
}

// licenseCacheKey generates a cache key for a workspace.
func licenseCacheKey(workspaceID string) string {
	return workspaceID
}

// IsFeatureEnabled checks if a feature is enabled for the workspace.
// Always enabled — no license restrictions.
func (*LicenseService) IsFeatureEnabled(_ context.Context, _ string, _ v1pb.PlanFeature) error {
	return nil
}

// IsFeatureEnabledForInstance checks if a feature is enabled for a specific instance.
// Always enabled — no license restrictions.
func (*LicenseService) IsFeatureEnabledForInstance(_ context.Context, _ string, _ v1pb.PlanFeature, _ *store.InstanceMessage) error {
	return nil
}

// IsInstanceEffectivelyActivated checks if an instance is effectively activated.
// Always true — no license restrictions.
func (*LicenseService) IsInstanceEffectivelyActivated(_ context.Context, _ string, _ *store.InstanceMessage) bool {
	return true
}

// IsUnifiedInstanceLicense checks if the workspace has a unified instance license.
// Always true — no license restrictions.
func (*LicenseService) IsUnifiedInstanceLicense(_ context.Context, _ string) bool {
	return true
}

// GetActivatedInstanceLimit returns the activated instance limit.
// Returns MaxInt32 — unlimited.
func (*LicenseService) GetActivatedInstanceLimit(_ context.Context, _ string) int {
	return math.MaxInt32
}

// GetInstanceLimit returns the instance limit.
// Returns MaxInt32 — unlimited.
func (*LicenseService) GetInstanceLimit(_ context.Context, _ string) int {
	return math.MaxInt32
}

// GetUserLimit returns the user (seat) limit.
// Returns MaxInt32 — unlimited.
func (*LicenseService) GetUserLimit(_ context.Context, _ string) int {
	return math.MaxInt32
}

// isUnifiedInstanceLimit checks if the instance limits are unified.
func isUnifiedInstanceLimit(instanceLimit, activatedLimit int) bool {
	return activatedLimit >= instanceLimit
}
