package handlers

import (
	"fmt"
	"net/http"
)

// Healthz is the liveness probe.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{"status":"ok"}`)
}
