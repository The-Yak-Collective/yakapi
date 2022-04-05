package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapmwkey int

const key zapmwkey = iota

type ResponseWriterWrapper struct {
	w          http.ResponseWriter
	written    int
	statusCode int
}

func (i *ResponseWriterWrapper) Write(buf []byte) (int, error) {
	written, err := i.w.Write(buf)
	i.written += written
	return written, err
}

func (i *ResponseWriterWrapper) WriteHeader(statusCode int) {
	i.statusCode = statusCode
	i.w.WriteHeader(statusCode)
}

func (i *ResponseWriterWrapper) Header() http.Header {
	return i.w.Header()
}

// New returns a new logging middleware using the provided *zap.Logger
func New(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// set incomplete request fields
			l := logger.With(
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("referrer", r.Referer()),
				zap.Time("start_time", start),
			)

			// store logger in context
			ctx := context.WithValue(r.Context(), key, l)

			// invoke next handler
			ww := ResponseWriterWrapper{w: w, statusCode: 200}
			next.ServeHTTP(&ww, r.WithContext(ctx))

			// get completed request fields
			l = l.With(
				zap.Duration("duration", time.Since(start)),
				zap.Int("status", ww.statusCode),
				zap.Int("bytes_written", ww.written),
			)

			logHTTPStatus(l, ww.statusCode)
		})
	}
}

func logHTTPStatus(l *zap.Logger, status int) {
	var msg string
	if msg = http.StatusText(status); msg == "" {
		msg = "unknown status " + strconv.Itoa(status)
	}

	var level zapcore.Level
	switch {
	case status >= 500:
		level = zapcore.ErrorLevel
	case status >= 400:
		level = zapcore.InfoLevel
	case status >= 300:
		level = zapcore.InfoLevel
	default:
		level = zapcore.InfoLevel
	}

	if ce := l.Check(level, msg); ce != nil {
		ce.Write()
	}
}

// Extract returns the *zap.Logger set by zapmw. If no logger is
// found in the context, zap.NewNop() is returned.
func Extract(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(key).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop()
}
