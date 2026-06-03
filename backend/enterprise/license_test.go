package enterprise

import (
	"math"
	"testing"

	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
)

func TestIsUnifiedInstanceLimit(t *testing.T) {
	tests := []struct {
		name           string
		instanceLimit  int
		activatedLimit int
		want           bool
	}{
		{name: "equal finite caps", instanceLimit: 10, activatedLimit: 10, want: true},
		{name: "activated cap larger than registration cap", instanceLimit: 10, activatedLimit: 20, want: true},
		{name: "split cap", instanceLimit: 50, activatedLimit: 20, want: false},
		{name: "unlimited both sides", instanceLimit: math.MaxInt, activatedLimit: math.MaxInt, want: true},
		{name: "unlimited registration finite activation", instanceLimit: math.MaxInt, activatedLimit: 20, want: false},
		{name: "finite registration unlimited activation", instanceLimit: 20, activatedLimit: math.MaxInt, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnifiedInstanceLimit(tt.instanceLimit, tt.activatedLimit); got != tt.want {
				t.Fatalf("isUnifiedInstanceLimit(%d, %d) = %v, want %v", tt.instanceLimit, tt.activatedLimit, got, tt.want)
			}
		})
	}
}

func TestLicenseServiceAllFeaturesEnabled(t *testing.T) {
	svc := NewLicenseService()

	// IsFeatureEnabled should always return nil.
	if err := svc.IsFeatureEnabled(nil, "any-workspace", v1pb.PlanFeature_FEATURE_DATA_MASKING); err != nil {
		t.Fatalf("IsFeatureEnabled should always return nil, got: %v", err)
	}

	// IsFeatureEnabledForInstance should always return nil.
	if err := svc.IsFeatureEnabledForInstance(nil, "any-workspace", v1pb.PlanFeature_FEATURE_DATA_MASKING, nil); err != nil {
		t.Fatalf("IsFeatureEnabledForInstance should always return nil, got: %v", err)
	}

	// IsInstanceEffectivelyActivated should always return true.
	if !svc.IsInstanceEffectivelyActivated(nil, "any-workspace", nil) {
		t.Fatal("IsInstanceEffectivelyActivated should always return true")
	}

	// IsUnifiedInstanceLicense should always return true.
	if !svc.IsUnifiedInstanceLicense(nil, "any-workspace") {
		t.Fatal("IsUnifiedInstanceLicense should always return true")
	}

	// Instance limits should be MaxInt32.
	if svc.GetInstanceLimit(nil, "any-workspace") != math.MaxInt32 {
		t.Fatal("GetInstanceLimit should return MaxInt32")
	}
	if svc.GetActivatedInstanceLimit(nil, "any-workspace") != math.MaxInt32 {
		t.Fatal("GetActivatedInstanceLimit should return MaxInt32")
	}

	// User limit should be MaxInt32.
	if svc.GetUserLimit(nil, "any-workspace") != math.MaxInt32 {
		t.Fatal("GetUserLimit should return MaxInt32")
	}
}

func TestCreateLicenseUsesEqualInstanceClaims(t *testing.T) {
	claims := newLicenseClaims(&LicenseParams{
		Plan:        v1pb.PlanType_ENTERPRISE.String(),
		Seats:       5,
		Instances:   10,
		WorkspaceID: "test-workspace",
	})
	if claims.Instances != 10 {
		t.Fatalf("Instances = %d, want 10", claims.Instances)
	}
	if claims.ActiveInstances != 10 {
		t.Fatalf("ActiveInstances = %d, want 10", claims.ActiveInstances)
	}
}
