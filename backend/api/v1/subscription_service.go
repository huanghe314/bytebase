package v1

import (
	"context"

	"connectrpc.com/connect"

	"github.com/bytebase/bytebase/backend/component/config"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
)

// SubscriptionService implements the subscription service.
type SubscriptionService struct {
	v1connect.UnimplementedSubscriptionServiceHandler
	profile *config.Profile
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(profile *config.Profile) *SubscriptionService {
	return &SubscriptionService{
		profile: profile,
	}
}

func enterpriseSubscription() *v1pb.Subscription {
	return &v1pb.Subscription{
		Plan:      v1pb.PlanType_ENTERPRISE,
		Seats:     2147483647,
		Instances: 2147483647,
	}
}

// GetSubscription gets the subscription.
func (s *SubscriptionService) GetSubscription(_ context.Context, _ *connect.Request[v1pb.GetSubscriptionRequest]) (*connect.Response[v1pb.Subscription], error) {
	return connect.NewResponse(enterpriseSubscription()), nil
}

// UpdateSubscription updates the subscription license.
func (s *SubscriptionService) UpdateSubscription(_ context.Context, _ *connect.Request[v1pb.UpdateSubscriptionRequest]) (*connect.Response[v1pb.Subscription], error) {
	return connect.NewResponse(enterpriseSubscription()), nil
}
