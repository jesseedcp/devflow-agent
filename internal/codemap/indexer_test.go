package codemap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexGoFactsExtractsFunctionsTypesMethodsTestsAndRouteStrings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "coupon", "service.go"), `package coupon

type Service struct{}

func (s Service) CheckEligibility(userID string) bool {
	route := "/coupon/claim"
	_ = route
	return userID != ""
}
`)
	writeFile(t, filepath.Join(root, "internal", "coupon", "service_test.go"), `package coupon

import "testing"

func TestCheckEligibilityInactiveUser(t *testing.T) {}
`)
	idx, err := IndexGoFacts(root)
	if err != nil {
		t.Fatalf("IndexGoFacts returned error: %v", err)
	}
	assertFact(t, idx, "type", "Service")
	assertFact(t, idx, "method", "CheckEligibility")
	assertFact(t, idx, "route", "/coupon/claim")
	assertFact(t, idx, "test", "TestCheckEligibilityInactiveUser")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFact(t *testing.T, idx Index, kind, name string) {
	t.Helper()
	for _, fact := range idx.Facts {
		if fact.Kind == kind && fact.Name == name {
			return
		}
	}
	t.Fatalf("missing fact kind=%s name=%s in %#v", kind, name, idx.Facts)
}

func TestIndexGoFactsSkipsGeneratedAndVendorDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "coupon", "service.go"), `package coupon
func CheckEligibility() bool { return true }
`)
	writeFile(t, filepath.Join(root, "vendor", "example.com", "coupon", "vendor.go"), `package coupon
func VendorCoupon() {}
`)
	writeFile(t, filepath.Join(root, ".devflow", "codemap", "generated.go"), `package generated
func GeneratedCoupon() {}
`)
	writeFile(t, filepath.Join(root, "tmp", "scratch.go"), `package tmp
func ScratchCoupon() {}
`)
	writeFile(t, filepath.Join(root, "temp", "scratch.go"), `package temp
func TempCoupon() {}
`)
	writeFile(t, filepath.Join(root, ".cache", "scratch.go"), `package cache
func CacheCoupon() {}
`)

	idx, err := IndexGoFacts(root)
	if err != nil {
		t.Fatalf("IndexGoFacts returned error: %v", err)
	}
	assertFact(t, idx, "func", "CheckEligibility")
	assertNoFact(t, idx, "VendorCoupon")
	assertNoFact(t, idx, "GeneratedCoupon")
	assertNoFact(t, idx, "ScratchCoupon")
	assertNoFact(t, idx, "TempCoupon")
	assertNoFact(t, idx, "CacheCoupon")
}

func assertNoFact(t *testing.T, idx Index, name string) {
	t.Helper()
	for _, fact := range idx.Facts {
		if fact.Name == name {
			t.Fatalf("unexpected fact name=%s in %#v", name, idx.Facts)
		}
	}
}
