package demandflow

import "testing"

func TestParseReleaseControlStages(t *testing.T) {
	for _, value := range []string{"deployment", "observation", "rollback"} {
		if _, err := ParseStage(value); err != nil {
			t.Fatalf("ParseStage(%q) returned error: %v", value, err)
		}
	}
}
