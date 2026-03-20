// Package enterprise implements the enterprise license service.
package enterprise

import (
	"context"
	"crypto/rsa"
	_ "embed"
	"encoding/json"
	"log/slog"
	"math"
	"slices"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

//go:embed plan.yaml
var planConfigStr string

var userLimitValues = map[v1pb.PlanType]int{}
var instanceLimitValues = map[v1pb.PlanType]int{}

// planFeatureMatrix maps plans to their available features
var planFeatureMatrix = make(map[v1pb.PlanType]map[v1pb.PlanFeature]bool)

var defaultFreeSubscription = &v1pb.Subscription{
	Plan: v1pb.PlanType_FREE,
}

func init() {
	// First unmarshal YAML to a generic map, then convert to JSON for protojson
	var yamlData map[string]any
	if err := yaml.Unmarshal([]byte(planConfigStr), &yamlData); err != nil {
		panic("failed to unmarshal plan.yaml: " + err.Error())
	}

	// Convert YAML data to JSON bytes
	jsonBytes, err := json.Marshal(yamlData)
	if err != nil {
		panic("failed to convert plan.yaml to JSON: " + err.Error())
	}

	conf := &v1pb.PlanConfig{}
	//nolint:forbidigo
	if err := protojson.Unmarshal(jsonBytes, conf); err != nil {
		panic("failed to unmarshal plan config proto: " + err.Error())
	}

	for _, plan := range conf.Plans {
		userLimitValues[plan.Type] = int(plan.MaximumSeatCount)
		instanceLimitValues[plan.Type] = int(plan.MaximumInstanceCount)

		planFeatureMatrix[plan.Type] = make(map[v1pb.PlanFeature]bool)
		for _, feature := range plan.Features {
			planFeatureMatrix[plan.Type][feature] = true
		}
	}
}

// Config is the API message for enterprise config.
type Config struct {
	// PublicKey is the parsed RSA public key.
	PublicKey *rsa.PublicKey
	// Version is the JWT key version.
	Version string
	// Issuer is the license issuer, it should always be "bytebase".
	Issuer string
	// Audience is the license audience, it should always be "bb.license".
	Audience string
	// Mode can be "prod" or "dev"
	Mode common.ReleaseMode
}

const (
	// keyID is the license key version.
	keyID = "v1"
	// issuer is the license issuer.
	issuer = "bytebase"
	// audience is the license token audience.
	audience = "bb.license"
)

// NewConfig will create a new enterprise config instance.
// Modified: Skip reading key files since license validation is bypassed.
func NewConfig(mode common.ReleaseMode) (*Config, error) {
	return &Config{
		PublicKey: nil, // Not used anymore
		Version:   keyID,
		Issuer:    issuer,
		Audience:  audience,
		Mode:      mode,
	}, nil
}

// replicaActiveWindow is the time window for considering a replica active.
// Replicas without heartbeats within this window are considered inactive.
// This should be at least 3x the heartbeat interval (10s) to tolerate missed heartbeats.
const replicaActiveWindow = 30 * time.Second

type replicaCacheState struct {
	replicaCount int
	loadedAt     time.Time
}

// LicenseService is the service for enterprise license.
type LicenseService struct {
	store        *store.Store
	config       *Config
	sfGroup      singleflight.Group
	cache        *expirable.LRU[string, *v1pb.Subscription]
	replicaCache atomic.Pointer[replicaCacheState]
}

// claims creates a struct that will be encoded to a JWT.
// We add jwt.RegisteredClaims as an embedded type, to provide fields such as name.
type claims struct {
	ActiveInstances int    `json:"instanceCount"`
	Instances       int    `json:"instance"`
	Seats           int    `json:"seat"`
	HA              bool   `json:"ha"`
	Trialing        bool   `json:"trialing"`
	Plan            string `json:"plan"`
	OrgName         string `json:"orgName"`
	WorkspaceID     string `json:"workspaceId"`
	jwt.RegisteredClaims
}

// NewLicenseService will create a new enterprise license service.
func NewLicenseService(mode common.ReleaseMode, store *store.Store) (*LicenseService, error) {
	config, err := NewConfig(mode)
	if err != nil {
		return nil, err
	}

	service := &LicenseService{
		store:  store,
		config: config,
		cache:  expirable.NewLRU[string, *v1pb.Subscription](1, nil, 1*time.Minute),
	}
	service.replicaCache.Store(&replicaCacheState{
		replicaCount: 1,
		loadedAt:     time.Now(),
	})

	return service, nil
}

const (
	cacheKey = "license"
)

// LoadSubscription will load subscription.
// If there is no license, we will return a free plan subscription without expiration time.
// If there is expired license, we will return a free plan subscription with the expiration time of the expired license.
func (s *LicenseService) LoadSubscription(ctx context.Context, workspaceID string) *v1pb.Subscription {
	// Fast path: cache hit (TTL handled automatically by expirable.LRU)
	if sub, ok := s.cache.Get(cacheKey); ok {
		return sub
	}

	// Slow path: load from DB with singleflight to prevent thundering herd
	v, _, _ := s.sfGroup.Do(cacheKey, func() (any, error) {
		// Double check after entering singleflight
		if sub, ok := s.cache.Get(cacheKey); ok {
			return sub, nil
		}

		subscription := s.loadSubscriptionFromDB(ctx, workspaceID)
		s.cache.Add(cacheKey, subscription)
		return subscription, nil
	})

	if sub, ok := v.(*v1pb.Subscription); ok {
		return sub
	}
	return defaultFreeSubscription
}

// loadSubscriptionFromDB loads subscription from database.
// Modified: Always return an enterprise subscription with unlimited access.
// No expiration time to suppress license expiration warnings.
func (s *LicenseService) loadSubscriptionFromDB(ctx context.Context, workspaceID string) *v1pb.Subscription {
	return &v1pb.Subscription{
		Plan:            v1pb.PlanType_ENTERPRISE,
		ActiveInstances: math.MaxInt32,
		Instances:       math.MaxInt32,
		Seats:           math.MaxInt32,
		Ha:              true,
		// ExpiresTime: nil - no expiration to suppress warnings
	}
}

func isExpired(sub *v1pb.Subscription) bool {
	if sub == nil {
		return false
	}
	return sub.ExpiresTime != nil && sub.ExpiresTime.AsTime().Before(time.Now())
}

// GetEffectivePlan gets the effective plan.
// Modified: Always return ENTERPRISE in dev mode.
func (s *LicenseService) GetEffectivePlan(ctx context.Context, workspaceID string) v1pb.PlanType {
	// Dev mode: always return ENTERPRISE
	if s.config.Mode == common.ReleaseModeDev {
		return v1pb.PlanType_ENTERPRISE
	}
	return s.LoadSubscription(ctx, workspaceID).Plan
}

// IsFeatureEnabled returns whether a feature is enabled.
// Modified: Always return nil to enable all features without license check.
func (s *LicenseService) IsFeatureEnabled(ctx context.Context, workspaceID string, f v1pb.PlanFeature) error {
	return nil
}

// IsFeatureEnabledForInstance returns whether a feature is enabled for the instance.
// Modified: Always return nil to enable all features for all instances.
func (s *LicenseService) IsFeatureEnabledForInstance(ctx context.Context, workspaceID string, f v1pb.PlanFeature, instance *store.InstanceMessage) error {
	return nil
}

// GetActivatedInstanceLimit returns the activated instance limit for the current subscription.
// Modified: Always return unlimited.
func (s *LicenseService) GetActivatedInstanceLimit(ctx context.Context, workspaceID string) int {
	return math.MaxInt
}

// GetUserLimit gets the user limit value for the plan.
// Modified: Always return unlimited.
func (s *LicenseService) GetUserLimit(ctx context.Context, workspaceID string) int {
	return math.MaxInt
}

// GetInstanceLimit gets the instance limit value for the plan.
// Modified: Always return unlimited.
func (s *LicenseService) GetInstanceLimit(ctx context.Context, workspaceID string) int {
	return math.MaxInt
}

// StoreLicense will store license into file.
// Modified: Skip license validation since all features are enabled.
func (s *LicenseService) StoreLicense(ctx context.Context, workspaceID string, license string) error {
	// Skip validation - all features enabled regardless of license
	if err := s.store.UpdateLicense(ctx, workspaceID, license); err != nil {
		return err
	}

	// Invalidate cache
	s.cache.Remove(cacheKey)

	return nil
}

// GetAuditLogRetentionDays returns the audit log retention period in days for the current plan.
// Modified: Always return -1 for unlimited retention.
func (s *LicenseService) GetAuditLogRetentionDays() int {
	return -1 // Unlimited
}

// GetAuditLogRetentionCutoff returns the earliest timestamp for accessible audit logs.
// Returns nil for unlimited retention (ENTERPRISE plan) or no access (FREE plan).
func (s *LicenseService) GetAuditLogRetentionCutoff() *time.Time {
	days := s.GetAuditLogRetentionDays()
	if days <= 0 {
		return nil // Either no access (0) or unlimited (-1)
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return &cutoff
}

// CountActiveReplicas returns the count of active replicas.
// A replica is considered active if it has sent a heartbeat within the last 30 seconds.
// Returns at least 1 (the current replica is always counted).
func (s *LicenseService) CountActiveReplicas(ctx context.Context) int {
	if state := s.replicaCache.Load(); state != nil && time.Since(state.loadedAt) < replicaActiveWindow {
		return state.replicaCount
	}

	count, err := s.store.CountActiveReplicas(ctx, replicaActiveWindow)
	if err != nil {
		slog.Warn("failed to count active replicas", log.BBError(err))
		return 1
	}

	if count < 1 {
		count = 1
	}
	s.replicaCache.Store(&replicaCacheState{
		replicaCount: count,
		loadedAt:     time.Now(),
	})

	return count
}

// CheckReplicaLimit checks if the current replica count exceeds the allowed limit.
// Modified: Always allow multiple replicas.
func (s *LicenseService) CheckReplicaLimit(ctx context.Context) error {
	return nil
}

func (s *LicenseService) parseLicense(license, workspaceID string) (*v1pb.Subscription, error) {
	claim := &claims{}
	token, err := jwt.ParseWithClaims(license, claim, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, common.Errorf(common.Invalid, "unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok || kid != s.config.Version {
			return nil, common.Errorf(common.Invalid, "version '%v' is not valid. expect %s", token.Header["kid"], s.config.Version)
		}

		return s.config.PublicKey, nil
	})
	if err != nil {
		return nil, common.Wrap(err, common.Invalid)
	}

	if !token.Valid {
		return nil, common.Errorf(common.Invalid, "invalid token")
	}

	if s.config.Issuer != claim.Issuer {
		return nil, common.Errorf(common.Invalid, "iss is not valid, expect %s but found '%v'", s.config.Issuer, claim.Issuer)
	}
	if !slices.Contains(claim.Audience, s.config.Audience) {
		return nil, common.Errorf(common.Invalid, "aud is not valid, expect %s but found '%v'", s.config.Audience, claim.Audience)
	}

	v, ok := v1pb.PlanType_value[claim.Plan]
	if !ok {
		return nil, common.Errorf(common.Invalid, "plan type %q is not valid", claim.Plan)
	}
	planType := v1pb.PlanType(v)

	if claim.WorkspaceID != "" && planType == v1pb.PlanType_ENTERPRISE && !claim.Trialing {
		if workspaceID != claim.WorkspaceID {
			return nil, common.Errorf(common.Invalid, "the workspace id not match")
		}
	}

	switch planType {
	case v1pb.PlanType_FREE, v1pb.PlanType_TEAM, v1pb.PlanType_ENTERPRISE:
	default:
		return nil, errors.Errorf("plan %q is not valid, expect %s or %s",
			planType.String(),
			v1pb.PlanType_TEAM.String(),
			v1pb.PlanType_ENTERPRISE.String(),
		)
	}

	var expiresTime *timestamppb.Timestamp
	if claim.ExpiresAt != nil && !claim.ExpiresAt.IsZero() {
		expiresTime = timestamppb.New(claim.ExpiresAt.Time)
	}
	if expiresTime != nil && expiresTime.AsTime().Before(time.Now()) {
		return nil, errors.Errorf("license has expired at %v", expiresTime.AsTime())
	}

	return &v1pb.Subscription{
		ActiveInstances: int32(claim.ActiveInstances),
		Instances:       int32(claim.Instances),
		Seats:           int32(claim.Seats),
		ExpiresTime:     expiresTime,
		Plan:            planType,
		Trialing:        claim.Trialing,
		OrgName:         claim.OrgName,
		Ha:              claim.HA,
	}, nil
}
