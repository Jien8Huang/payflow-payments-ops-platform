package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/payflow/payflow-app/internal/auth"
)

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.code == 0 {
		s.code = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) status() int {
	if s.code == 0 {
		return http.StatusOK
	}
	return s.code
}

// RequestLogger emits one structured line per request (R22).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()
		defer func() {
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status()),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			}
			if rid := auth.RequestID(r.Context()); rid != "" {
				attrs = append(attrs, slog.String("request_id", rid))
			}
			if tid, ok := auth.TenantID(r.Context()); ok {
				attrs = append(attrs, slog.String("tenant_id", tid.String()))
			}
			if pk, ok := auth.Principal(r.Context()); ok {
				attrs = append(attrs, slog.String("principal", string(pk)))
			}
			slog.LogAttrs(r.Context(), slog.LevelInfo, "http_request", attrs...)
		}()
		next.ServeHTTP(rec, r)
	})
}
