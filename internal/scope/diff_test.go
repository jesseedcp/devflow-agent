package scope

import "testing"

func TestCompareChangedFilesToDeclaration(t *testing.T) {
	decl := Declaration{SourceFiles: []string{"internal/coupon/service.go"}, TestFiles: []string{"internal/coupon/service_test.go"}}
	result := CompareChangedFiles(decl, []string{"internal/coupon/service.go", "README.md"})
	if len(result.InScope) != 1 || result.InScope[0] != "internal/coupon/service.go" {
		t.Fatalf("InScope = %#v", result.InScope)
	}
	if len(result.OutOfScope) != 1 || result.OutOfScope[0] != "README.md" {
		t.Fatalf("OutOfScope = %#v", result.OutOfScope)
	}
	if len(result.MissingTests) != 1 || result.MissingTests[0] != "internal/coupon/service_test.go" {
		t.Fatalf("MissingTests = %#v", result.MissingTests)
	}
}

func TestCompareChangedFilesSeesDeclaredTest(t *testing.T) {
	decl := Declaration{SourceFiles: []string{"internal/coupon/service.go"}, TestFiles: []string{"internal/coupon/service_test.go"}}
	result := CompareChangedFiles(decl, []string{"internal/coupon/service.go", "internal/coupon/service_test.go"})
	if len(result.OutOfScope) != 0 || len(result.MissingTests) != 0 {
		t.Fatalf("result = %#v, want all in scope and no missing tests", result)
	}
}
