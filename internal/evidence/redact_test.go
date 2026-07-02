package evidence

import (
	"strings"
	"testing"
)

func TestRedactSensitiveText(t *testing.T) {
	input := strings.Join([]string{
		"Authorization: Bearer secret-token",
		"Cookie: sid=abc; theme=dark",
		"https://api.example.test/coupon?token=abc&user=1",
		`{"api_key":"abc","password":"pw","ok":true}`,
	}, "\n")
	got := Redact(input)
	for _, forbidden := range []string{"secret-token", "sid=abc", "token=abc", `"api_key":"abc"`, `"password":"pw"`} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redacted text leaked %q:\n%s", forbidden, got)
		}
	}
	for _, want := range []string{"Authorization: Bearer ***", "Cookie: ***", "token=***", `"api_key":"***"`, `"password":"***"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted text missing %q:\n%s", want, got)
		}
	}
}

func TestExcerptRedactsAndTruncates(t *testing.T) {
	got := Excerpt("token=abc "+strings.Repeat("x", 5000), 64)
	if strings.Contains(got, "abc") {
		t.Fatalf("excerpt leaked token: %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("excerpt missing truncation marker: %q", got)
	}
	if len(got) > 128 {
		t.Fatalf("excerpt too long: %d", len(got))
	}
}
