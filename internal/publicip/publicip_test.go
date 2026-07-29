package publicip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10\n"))
	}))
	defer srv.Close()

	ip, err := Detect(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("got %q", ip)
	}
}

func TestDetectInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-an-ip"))
	}))
	defer srv.Close()
	if _, err := Detect(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error")
	}
}

func TestLocalTowardLoopback(t *testing.T) {
	ip, err := LocalToward("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if ip != "127.0.0.1" {
		t.Fatalf("got %q", ip)
	}
}
