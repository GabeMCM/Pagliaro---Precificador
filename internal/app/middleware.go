package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/id"
)

type requestContextKey string

const requestMetaContextKey requestContextKey = "request_meta"

type requestMeta struct {
	RequestID string
	APIKeyID  string
	Role      string
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func (a *App) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		meta := &requestMeta{RequestID: id.New("req")}
		ctx := context.WithValue(r.Context(), requestMetaContextKey, meta)
		recorder := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r.WithContext(ctx))
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		duration := time.Since(start)
		log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d role=%s api_key_id=%s",
			meta.RequestID, r.Method, r.URL.Path, recorder.status, duration.Milliseconds(), meta.Role, meta.APIKeyID)

		requestLog := domain.RequestLog{
			ID:         id.New("log"),
			RequestID:  meta.RequestID,
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     recorder.status,
			DurationMS: duration.Milliseconds(),
			APIKeyID:   meta.APIKeyID,
			Role:       meta.Role,
			RemoteAddr: r.RemoteAddr,
			CreatedAt:  time.Now().UTC(),
		}
		if err := a.store.SaveRequestLog(context.Background(), requestLog); err != nil {
			log.Printf("salvando request log: %v", err)
		}
	})
}

func currentRequestMeta(ctx context.Context) *requestMeta {
	meta, _ := ctx.Value(requestMetaContextKey).(*requestMeta)
	return meta
}
