package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbes(t *testing.T) {
	for _, test := range []struct {
		path, method string
		fail         bool
		status       int
	}{{"/healthz", "GET", true, 200}, {"/readyz", "GET", false, 200}, {"/readyz", "GET", true, 503}, {"/missing", "GET", false, 404}, {"/healthz", "POST", false, 405}} {
		t.Run(test.path+test.method, func(t *testing.T) {
			h := Router(func(context.Context) error {
				if test.fail {
					return errors.New("private-dsn")
				}
				return nil
			}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(test.method, test.path, nil))
			if w.Code != test.status {
				t.Fatalf("status %d", w.Code)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing request id")
			}
			if strings.Contains(w.Body.String(), "private-dsn") {
				t.Fatal("leaked database error")
			}
			if w.Code >= 400 {
				var body Error
				if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.RequestID == "" {
					t.Fatal("invalid error envelope")
				}
			}
		})
	}
}
