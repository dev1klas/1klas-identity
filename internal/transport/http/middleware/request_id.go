package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

const headerRequestID = "X-Request-Id"

// RequestID preserves or generates an X-Request-Id header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = uuid.New().String()
		}
		ctx := WithRequestID(r.Context(), id)
		w.Header().Set(headerRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
