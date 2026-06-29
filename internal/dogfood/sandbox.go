package dogfood

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jesseedcp/devflow-agent/internal/quality"
)

const liveSandboxModule = "devflow-live-dogfood"

type Sandbox struct {
	Root            string
	RepoRoot        string
	DemandRoot      string
	QualityCommands []quality.Command
}

func CreateLiveSandbox(root string) (Sandbox, error) {
	if root == "" {
		temp, err := os.MkdirTemp("", "devflow-live-dogfood-*")
		if err != nil {
			return Sandbox{}, fmt.Errorf("create live dogfood root: %w", err)
		}
		root = temp
	}
	root = filepath.Clean(root)
	repoRoot := filepath.Join(root, "repo")
	demandRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(repoRoot, "coupon"), 0o755); err != nil {
		return Sandbox{}, fmt.Errorf("create sandbox repo: %w", err)
	}
	if err := os.MkdirAll(demandRoot, 0o755); err != nil {
		return Sandbox{}, fmt.Errorf("create sandbox artifact root: %w", err)
	}
	files := map[string]string{
		filepath.Join(repoRoot, "go.mod"):                        sandboxGoMod(),
		filepath.Join(repoRoot, "coupon", "eligibility.go"):      sandboxEligibility(),
		filepath.Join(repoRoot, "coupon", "eligibility_test.go"): sandboxEligibilityTest(),
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return Sandbox{}, fmt.Errorf("write sandbox file %s: %w", path, err)
		}
	}
	return Sandbox{
		Root:       root,
		RepoRoot:   repoRoot,
		DemandRoot: demandRoot,
		QualityCommands: []quality.Command{{
			Name: "go",
			Args: []string{"test", "./...", "-count=1", "-timeout", "2m"},
		}},
	}, nil
}

func sandboxGoMod() string {
	return "module " + liveSandboxModule + "\n\ngo 1.25\n"
}

func sandboxEligibility() string {
	return `package coupon

import "time"

type User struct {
	ID     string
	Active bool
}

type Coupon struct {
	ID        string
	ExpiresAt time.Time
}

type Claim struct {
	UserID   string
	CouponID string
}

func Eligible(user User, coupon Coupon, existing []Claim, now time.Time) (bool, string) {
	return false, "not implemented"
}
`
}

func sandboxEligibilityTest() string {
	return `package coupon

import (
	"testing"
	"time"
)

func TestEligibleAllowsActiveUserWithFreshCoupon(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	ok, reason := Eligible(User{ID: "u1", Active: true}, Coupon{ID: "c1", ExpiresAt: now.Add(time.Hour)}, nil, now)
	if !ok || reason != "eligible" {
		t.Fatalf("Eligible active user = (%v, %q), want (true, eligible)", ok, reason)
	}
}

func TestEligibleRejectsInactiveUser(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	ok, reason := Eligible(User{ID: "u1", Active: false}, Coupon{ID: "c1", ExpiresAt: now.Add(time.Hour)}, nil, now)
	if ok || reason != "inactive user" {
		t.Fatalf("Eligible inactive user = (%v, %q), want (false, inactive user)", ok, reason)
	}
}

func TestEligibleRejectsExpiredCoupon(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	ok, reason := Eligible(User{ID: "u1", Active: true}, Coupon{ID: "c1", ExpiresAt: now.Add(-time.Minute)}, nil, now)
	if ok || reason != "expired coupon" {
		t.Fatalf("Eligible expired coupon = (%v, %q), want (false, expired coupon)", ok, reason)
	}
}

func TestEligibleRejectsDuplicateClaim(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	claims := []Claim{{UserID: "u1", CouponID: "c1"}}
	ok, reason := Eligible(User{ID: "u1", Active: true}, Coupon{ID: "c1", ExpiresAt: now.Add(time.Hour)}, claims, now)
	if ok || reason != "already claimed" {
		t.Fatalf("Eligible duplicate claim = (%v, %q), want (false, already claimed)", ok, reason)
	}
}
`
}
