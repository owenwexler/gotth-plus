package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func serve(contentType, acceptEncoding string, status int) *httptest.ResponseRecorder {
	h := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(status)
		if status != http.StatusNoContent {
			io.WriteString(w, strings.Repeat("hello ", 200))
		}
	}))
	req := httptest.NewRequest("GET", "/", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCompress(t *testing.T) {
	want := strings.Repeat("hello ", 200)

	tests := []struct {
		name, contentType, accept, encoding string
		status                              int
	}{
		{"brotli preferred", "text/html; charset=utf-8", "gzip, br", "br", 200},
		{"gzip", "application/javascript", "gzip", "gzip", 200},
		{"q=0 refused", "text/html", "br;q=0, gzip;q=0", "", 200},
		{"no header", "text/html", "", "", 200},
		{"font skipped", "font/woff2", "br, gzip", "", 200},
		{"204 skipped", "text/html", "br", "", 204},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.contentType, tt.accept, tt.status)

			if got := rec.Header().Get("Content-Encoding"); got != tt.encoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tt.encoding)
			}
			if rec.Header().Get("Vary") != "Accept-Encoding" {
				t.Errorf("missing Vary: Accept-Encoding")
			}
			if tt.status == 204 {
				return
			}

			var body []byte
			switch tt.encoding {
			case "br":
				body, _ = io.ReadAll(brotli.NewReader(rec.Body))
			case "gzip":
				zr, err := gzip.NewReader(rec.Body)
				if err != nil {
					t.Fatal(err)
				}
				body, _ = io.ReadAll(zr)
			default:
				body = rec.Body.Bytes()
			}
			if string(body) != want {
				t.Errorf("decoded body mismatch (%d bytes)", len(body))
			}
			if tt.encoding != "" && rec.Header().Get("Content-Length") != "" {
				t.Errorf("Content-Length should be removed when compressing")
			}
		})
	}
}
