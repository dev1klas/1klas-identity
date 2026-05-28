package handlers

import (
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/transport/apispec"
)

// OpenAPI serves the embedded OpenAPI 3.0 spec.
func OpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(apispec.Spec)
}
