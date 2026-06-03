package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
)

func TestSubscription(t *testing.T) {
	a := require.New(t)
	ctx := context.Background()
	ctl := &controller{}
	ctx, err := ctl.StartServerWithExternalPg(ctx)
	a.NoError(err)
	defer ctl.Close(ctx)

	subscription, err := ctl.getSubscription(ctx)
	a.NoError(err)
	// All enterprise features are always enabled — GetSubscription always returns ENTERPRISE.
	a.Equal(v1pb.PlanType_ENTERPRISE, subscription.Plan)
}
