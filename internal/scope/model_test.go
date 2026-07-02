package scope

import (
	"strings"
	"testing"
)

func TestRenderAndParseDeclaration(t *testing.T) {
	decl := Declaration{
		SourceFiles: []string{"internal/coupon/service.go"},
		TestFiles:   []string{"internal/coupon/service_test.go"},
		OutOfScope:  []string{"internal/payments"},
	}
	text := RenderDeclaration("Coupon", decl)
	for _, want := range []string{"# Change Scope: Coupon", "## Source Files", "internal/coupon/service.go", "## Test Files", "internal/coupon/service_test.go", "## Out Of Scope", "internal/payments"} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered scope missing %q:\n%s", want, text)
		}
	}
	parsed := ParseDeclaration(text)
	if len(parsed.SourceFiles) != 1 || parsed.SourceFiles[0] != "internal/coupon/service.go" {
		t.Fatalf("SourceFiles = %#v", parsed.SourceFiles)
	}
	if len(parsed.TestFiles) != 1 || parsed.TestFiles[0] != "internal/coupon/service_test.go" {
		t.Fatalf("TestFiles = %#v", parsed.TestFiles)
	}
	if len(parsed.OutOfScope) != 1 || parsed.OutOfScope[0] != "internal/payments" {
		t.Fatalf("OutOfScope = %#v", parsed.OutOfScope)
	}
}
