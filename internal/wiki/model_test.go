package wiki

import "testing"

func TestCandidateKindValues(t *testing.T) {
	cases := []struct {
		kind CandidateKind
		want string
	}{
		{KindBusiness, "business"},
		{KindProcess, "process"},
		{KindArchive, "archive"},
	}
	for _, c := range cases {
		if string(c.kind) != c.want {
			t.Fatalf("kind = %q, want %q", c.kind, c.want)
		}
	}
}

func TestCandidateStatusValues(t *testing.T) {
	cases := []struct {
		status CandidateStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusPromoted, "promoted"},
		{StatusRejected, "rejected"},
	}
	for _, c := range cases {
		if string(c.status) != c.want {
			t.Fatalf("status = %q, want %q", c.status, c.want)
		}
	}
}