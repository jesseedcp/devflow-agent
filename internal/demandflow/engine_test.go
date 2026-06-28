package demandflow

import (
	"context"
	"testing"
)

func TestStaticRunnerCapturesRequests(t *testing.T) {
	t.Parallel()

	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageRequirements: {Text: "requirements body"},
	}}
	resp, err := runner.Run(context.Background(), RunnerRequest{
		Stage:    StageRequirements,
		Root:     t.TempDir(),
		DemandID: "add-coupon-check",
		Prompt:   "prompt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Text != "requirements body" {
		t.Fatalf("response = %q want requirements body", resp.Text)
	}
	if len(runner.Requests) != 1 || runner.Requests[0].DemandID != "add-coupon-check" {
		t.Fatalf("request not captured: %#v", runner.Requests)
	}
}
