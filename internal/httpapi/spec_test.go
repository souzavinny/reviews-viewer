package httpapi_test

import (
	"strings"
	"testing"

	"github.com/souzavinny/reviews-api/api"
)

// TestSpecCoversRoutes guards against drift between the hand-maintained OpenAPI
// spec and the handler's routes — the frontend mirrors this spec, so a silent
// mismatch would break it with no other signal. Stdlib string scan, no YAML dep.
//
// When a data route changes, update both api/openapi.yaml and this table.
func TestSpecCoversRoutes(t *testing.T) {
	spec := string(api.Spec)

	routes := []struct {
		method string
		path   string
		opID   string
	}{
		{"GET", "/healthz", "health"},
		{"GET", "/apps", "listApps"},
		{"POST", "/apps", "addApp"},
		{"DELETE", "/apps/{appID}", "removeApp"},
		{"GET", "/apps/{appID}/reviews", "getReviews"},
		{"GET", "/apps/{appID}/summary", "getSummary"},
	}

	for _, rt := range routes {
		if !strings.Contains(spec, "\n  "+rt.path+":") {
			t.Errorf("%s %s: path %q is missing from the OpenAPI spec", rt.method, rt.path, rt.path)
		}
		if !strings.Contains(spec, "operationId: "+rt.opID) {
			t.Errorf("%s %s: operationId %q is missing from the OpenAPI spec", rt.method, rt.path, rt.opID)
		}
	}

	if got := strings.Count(spec, "operationId:"); got != len(routes) {
		t.Errorf("spec declares %d operations, want %d — the documented route set drifted from the handlers", got, len(routes))
	}
}
