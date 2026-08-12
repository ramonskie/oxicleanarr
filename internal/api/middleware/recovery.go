package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// Recovery is a middleware that recovers from panics.
//
// It only writes the 500 response when the handler panicked before writing
// any response bytes; if headers were already committed (handler partially
// wrote a response), writing again would produce a superfluous WriteHeader
// warning and corrupt the body. In that case the connection is just closed.
// http.ErrAbortHandler (client disconnect) is re-raised so net/http can finish
// the aborted connection without writing to a dead socket.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			if err := recover(); err != nil {
				// Client disconnect mid-response; let net/http handle the
				// aborted connection instead of writing to a dead socket.
				if err == http.ErrAbortHandler {
					panic(err)
				}

				log.Error().
					Interface("error", err).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Stack().
					Msg("Panic recovered")

				// If any bytes were written or headers flushed, a second
				// WriteHeader/Write would be superfluous and corrupt the body.
				// Close instead. (Status() catches explicit WriteHeader/Write;
				// BytesWritten() catches Write-without-WriteHeader.)
				if ww.Status() != 0 || ww.BytesWritten() > 0 {
					return
				}

				ww.Header().Set("Content-Type", "application/json")
				ww.WriteHeader(http.StatusInternalServerError)
				ww.Write([]byte(`{"error": "Internal server error"}`))
			}
		}()

		next.ServeHTTP(ww, r)
	})
}
