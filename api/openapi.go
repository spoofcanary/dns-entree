package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiBytes []byte

// OpenAPISpec returns the embedded OpenAPI document. Tests use this to assert
// the binary always ships with a non-empty spec.
func OpenAPISpec() []byte { return openapiBytes }

func openapiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(openapiBytes)
}
