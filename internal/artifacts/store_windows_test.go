//go:build windows

package artifacts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDemandRejectsJunctionedDemandRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	junction := filepath.Join(root, ".devflow")

	t.Cleanup(func() {
		if err := os.Remove(junction); err != nil && !os.IsNotExist(err) {
			t.Fatalf("Remove(%s) returned error: %v", junction, err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Fatalf("outside directory missing after junction cleanup: %v", err)
		}
	})

	cmd := exec.Command("cmd", "/c", "mklink", "/J", junction, outside)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("mklink /J unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	store := NewStore(root)
	err = store.CreateDemand(testDemand("risk-flag"))
	if err == nil {
		t.Fatal("CreateDemand returned nil error")
	}
	if !strings.Contains(err.Error(), "reparse point") && !strings.Contains(err.Error(), "unsafe demand path") {
		t.Fatalf("CreateDemand error = %q, want reparse point or unsafe demand path", err)
	}

	outsideDemandDir := filepath.Join(outside, "demands", "risk-flag")
	if _, statErr := os.Stat(outsideDemandDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected no escaped demand directory, stat error = %v", statErr)
	}
}
