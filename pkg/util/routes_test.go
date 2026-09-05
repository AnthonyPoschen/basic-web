package util

import "testing"

func TestBindRoutePatternRejectsEmptyOrUnsafeParams(t *testing.T) {
	path, ok := bindRoutePattern("/plans/:game/:region", map[string]string{"game": "factorio", "region": "australia"})
	if ok == false || path != "/plans/factorio/australia" {
		t.Fatalf("bound path = %q ok=%v", path, ok)
	}
	if _, ok := bindRoutePattern("/plans/:game/:region", map[string]string{"game": "factorio"}); ok {
		t.Fatal("missing region was accepted")
	}
	if _, ok := bindRoutePattern("/plans/:game/:region", map[string]string{"game": "factorio/extra", "region": "australia"}); ok {
		t.Fatal("slash in param was accepted")
	}
}
