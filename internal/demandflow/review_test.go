package demandflow

import (
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type fakeReviewAdapter struct {
	comments []adapters.ReviewComment
	listErr  error
}

func (f fakeReviewAdapter) ListUnresolved(_ context.Context, _ adapters.ReviewRef) ([]adapters.ReviewComment, error) {
	return f.comments, f.listErr
}

func (f fakeReviewAdapter) Reply(_ context.Context, _ adapters.ReviewRef, _ string, _ string) error {
	return nil
}

func mrReviewOptions(adapter adapters.ReviewAdapter) ReviewOptions {
	return ReviewOptions{Adapter: adapter, Ref: adapters.ReviewRef{Project: "group/project", MergeRequest: "1"}}
}

func TestEngineMRReviewNoCommentsAdvancesToVerification(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(fakeReviewAdapter{}),
		Now:      fixedNow,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Verification) {
		t.Fatalf("state = %q want verification", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.ProgressFile); !strings.Contains(body, "No unresolved review comments") {
		t.Fatalf("progress.md = %q want review summary", body)
	}
}

func TestEngineMRReviewNonblockingCommentAdvances(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "nit: rename helper", Blocking: false},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Verification) {
		t.Fatalf("state = %q want verification", demand.State)
	}
}

func TestEngineMRReviewBlockingTestCommentRoutesToImplementation(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "missing test coverage", Blocking: true, FilePath: "main.go", Line: 10},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "implementation updates") {
		t.Fatalf("err = %v want implementation updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Implementation) {
		t.Fatalf("state = %q want implementation", demand.State)
	}
}

func TestEngineMRReviewRequiresAdapter(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "review adapter") {
		t.Fatalf("err = %v want review adapter error", err)
	}
}

func TestEngineMRReviewRequirementsCommentReturnsToRequirements(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "missing acceptance criteria", Blocking: true, Category: adapters.CommentRequirements},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "requirements updates") {
		t.Fatalf("err = %v want requirements updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.ReturnedToRequirements) {
		t.Fatalf("state = %q want returned_to_requirements", demand.State)
	}
	body := readArtifact(t, engine, artifacts.ProgressFile)
	if !strings.Contains(body, "MR Review Action Plan") || !strings.Contains(body, "returned_to_requirements") {
		t.Fatalf("progress.md missing action plan:\n%s", body)
	}
}

func TestEngineMRReviewPlanCommentReturnsToPlan(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "adapter boundary is wrong", Blocking: true, Category: adapters.CommentPlan},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "plan updates") {
		t.Fatalf("err = %v want plan updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.ReturnedToPlan) {
		t.Fatalf("state = %q want returned_to_plan", demand.State)
	}
}

func TestEngineMRReviewImplementationCommentReturnsToImplementation(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "nil handling is wrong", Blocking: true, Category: adapters.CommentImplementation},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "implementation updates") {
		t.Fatalf("err = %v want implementation updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Implementation) {
		t.Fatalf("state = %q want implementation", demand.State)
	}
}

type fakeCIGate struct {
	result adapters.CIResult
	err    error
}

func (f fakeCIGate) Check(context.Context, adapters.CIRef) (adapters.CIResult, error) {
	return f.result, f.err
}

func TestEngineMRReviewClearCommentsButPendingCIRemainsBlocked(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(fakeReviewAdapter{}),
		CIGate: CIGateOptions{
			Adapter: fakeCIGate{result: adapters.CIResult{
				Provider: "github",
				Repo:     "owner/repo",
				PR:       "42",
				Status:   adapters.CIStatusPending,
				Message:  "github ci pending",
				Checks:   []adapters.CICheck{{Name: "Go verification", Status: "queued"}},
			}},
			Ref: adapters.CIRef{Provider: "github", Repo: "owner/repo", PR: "42"},
		},
		Now: fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "ci gate blocked") {
		t.Fatalf("err = %v, want ci gate blocked", err)
	}
	updated, loadErr := engine.Store.LoadDemand("add-coupon-check")
	if loadErr != nil {
		t.Fatalf("LoadDemand returned error: %v", loadErr)
	}
	if workflow.State(updated.State) != workflow.MRReview {
		t.Fatalf("state = %s, want %s", updated.State, workflow.MRReview)
	}
	body := readArtifact(t, engine, artifacts.ProgressFile)
	if !strings.Contains(body, "## CI Gate") || !strings.Contains(body, "pending") {
		t.Fatalf("progress.md missing CI gate evidence:\n%s", body)
	}
}

func TestEngineMRReviewClearCommentsAndPassingCIAdvancesToVerification(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(fakeReviewAdapter{}),
		CIGate: CIGateOptions{
			Adapter: fakeCIGate{result: adapters.CIResult{
				Provider: "github",
				Repo:     "owner/repo",
				PR:       "42",
				Status:   adapters.CIStatusPassed,
				Checks:   []adapters.CICheck{{Name: "Go verification", Status: "completed", Conclusion: "success"}},
				Message:  "github ci passed",
			}},
			Ref: adapters.CIRef{Provider: "github", Repo: "owner/repo", PR: "42"},
		},
		Now: fixedNow,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	updated, loadErr := engine.Store.LoadDemand("add-coupon-check")
	if loadErr != nil {
		t.Fatalf("LoadDemand returned error: %v", loadErr)
	}
	if workflow.State(updated.State) != workflow.Verification {
		t.Fatalf("state = %s, want %s", updated.State, workflow.Verification)
	}
}
