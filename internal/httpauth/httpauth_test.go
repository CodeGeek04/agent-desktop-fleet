package httpauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := RequireToken("s3cret", ok)
	cases := map[string]int{
		"":                  http.StatusUnauthorized,
		"Bearer wrong":      http.StatusUnauthorized,
		"s3cret":            http.StatusUnauthorized,
		"Bearer s3cret":     http.StatusTeapot,
		"bearer s3cret":     http.StatusTeapot,
		"Bearer s3cret-and": http.StatusUnauthorized,
	}
	for header, want := range cases {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Authorization %q: status %d, want %d", header, rec.Code, want)
		}
	}
}

func TestNewTokenIsRandomHex(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewToken()
	if len(a) != 64 || a == b {
		t.Fatalf("tokens %q and %q", a, b)
	}
}
