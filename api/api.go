// Package api holds the OpenAPI contract for the reviews API. The spec is
// embedded so the server can serve it without reading from disk at runtime.
package api

import _ "embed"

// Spec is the raw OpenAPI 3.1 document (api/openapi.yaml).
//
//go:embed openapi.yaml
var Spec []byte
