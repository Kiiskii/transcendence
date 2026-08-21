package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The regression this pins lived on main for months as commented-out code:
// without the overrides the access log reports 200 and 0 bytes for every
// request, including the failures.
func TestResponseWriterRecordsWhatWasSent(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantSize   int
	}{
		{
			name: "explicit status and body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"Listing not found"}`))
			},
			wantStatus: http.StatusNotFound,
			wantSize:   29,
		},
		{
			name: "implicit 200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			},
			wantStatus: http.StatusOK,
			wantSize:   2,
		},
		{
			name: "size accumulates across writes",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("abc"))
				_, _ = w.Write([]byte("de"))
			},
			wantStatus: http.StatusOK,
			wantSize:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := &responseWriter{
				ResponseWriter: httptest.NewRecorder(),
				status:         http.StatusOK,
			}

			tt.handler(rw, httptest.NewRequest(http.MethodGet, "/", nil))

			if rw.status != tt.wantStatus {
				t.Errorf("status = %d, want %d", rw.status, tt.wantStatus)
			}
			if rw.size != tt.wantSize {
				t.Errorf("size = %d, want %d", rw.size, tt.wantSize)
			}
		})
	}
}

// The id has to reach the client too, or a bug report can't quote the value
// that appears in our logs.
func TestLoggerExposesTheRequestID(t *testing.T) {
	handler := Logger(slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings", nil)

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "" {
		t.Errorf("X-Request-ID = %q, want it unset when no id is in the context", got)
	}
}
