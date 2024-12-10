package http_adapters

import (
	"net/http"
	"runtime/debug"

	"log/slog"

	"github.com/x0k/medods-authentication-service/internal/lib/logger"
)

type statusCapturer struct {
	http.ResponseWriter
	status int
}

func (sc *statusCapturer) WriteHeader(status int) {
	sc.ResponseWriter.WriteHeader(status)
	sc.status = status
}

func Logging(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := statusCapturer{
			ResponseWriter: w,
		}
		next.ServeHTTP(&c, r)
		log.Info(
			r.Context(),
			"request",
			slog.String("method", r.Method),
			slog.String("url", r.RequestURI),
			slog.Int("status", c.status),
		)
	})
}

func Recover(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error(r.Context(), "panic", slog.String("stack", string(debug.Stack())), slog.Any("error", err))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
