package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/ramonskie/oxicleanarr/internal/utils"
)

// Logger is a middleware that logs HTTP requests
// Logs are written to web.log and tagged with component="web"
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			// Use debug level for polling/health check endpoints to reduce log noise
			isPollingEndpoint := r.URL.Path == "/api/sync/status" ||
				r.URL.Path == "/api/health" ||
				r.URL.Path == "/health"

			logger := utils.GetWebLogger()

			logEvent := logger.Info()
			if isPollingEndpoint {
				logEvent = logger.Debug()
			}

			logEvent.
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("duration_ms", time.Since(start)).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("HTTP request")
		}()

		next.ServeHTTP(ww, r)
	})
}
