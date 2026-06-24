package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestStartCreatesDemandWorkspace(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	err := Run([]string{
		"start",
		"--root", root,
		"--title", "Add coupon check",
		"--description", "Only active members can claim coupons",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "add-coupon-check") {
		t.Fatalf("stdout = %q, want created demand slug", output)
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", "add-coupon-check", "requirements.md")); err != nil {
		t.Fatalf("requirements workspace missing: %v", err)
	}

	demand, err := artifacts.NewStore(root).LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if demand.State != string(workflow.Created) {
		t.Fatalf("demand state = %q, want %q", demand.State, workflow.Created)
	}
}

func TestSlugifyGeneratesStableSafeIDForNonASCIIOnlyTitles(t *testing.T) {
	t.Parallel()

	first := slugify("新增优惠券校验")
	second := slugify("新增优惠券校验")

	if first == "demand" {
		t.Fatalf("slugify returned fallback %q, want hashed demand id", first)
	}
	if !regexp.MustCompile(`^demand-[0-9a-f]{12}$`).MatchString(first) {
		t.Fatalf("slugify = %q, want demand-<12hex>", first)
	}
	if first != second {
		t.Fatalf("slugify not stable: first %q second %q", first, second)
	}
}

func TestSlugifyDistinguishesDifferentNonASCIITitles(t *testing.T) {
	t.Parallel()

	left := slugify("新增优惠券校验")
	right := slugify("新增风险标记")
	if left == right {
		t.Fatalf("slugify collision: left %q right %q", left, right)
	}
}

func TestSlugifyAppendsHashForMixedLanguageTitles(t *testing.T) {
	t.Parallel()

	slug := slugify("新增 coupon 校验")
	if !regexp.MustCompile(`^coupon-[0-9a-f]{12}$`).MatchString(slug) {
		t.Fatalf("slugify = %q, want coupon-<12hex>", slug)
	}
}

func TestStartCreatesDistinctWorkspacesForDifferentChineseTitles(t *testing.T) {
	root := t.TempDir()
	var firstStdout bytes.Buffer
	var secondStdout bytes.Buffer

	firstTitle := "新增优惠券校验"
	secondTitle := "新增风险标记"

	if err := Run([]string{"start", "--root", root, "--title", firstTitle}, &firstStdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := Run([]string{"start", "--root", root, "--title", secondTitle}, &secondStdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("second start: %v", err)
	}

	firstID := strings.TrimSpace(strings.TrimPrefix(firstStdout.String(), "Created demand "))
	firstID = strings.SplitN(firstID, " under ", 2)[0]
	secondID := strings.TrimSpace(strings.TrimPrefix(secondStdout.String(), "Created demand "))
	secondID = strings.SplitN(secondID, " under ", 2)[0]

	if firstID == secondID {
		t.Fatalf("expected different ids, got %q and %q", firstID, secondID)
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", firstID, "requirements.md")); err != nil {
		t.Fatalf("first requirements workspace missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", secondID, "requirements.md")); err != nil {
		t.Fatalf("second requirements workspace missing: %v", err)
	}
}
