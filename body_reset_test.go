package retryablehttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSetBodyNilClearsPreviousRequestBody(t *testing.T) {
	bodies := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		bodies <- string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	payload := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(payload, []byte("old payload"), 0600); err != nil {
		t.Fatal(err)
	}
	req, err := NewRequest(http.MethodPost, server.URL, ReaderFunc(func() (io.Reader, error) {
		return os.Open(payload)
	}))
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient()
	client.RetryMax = 0
	client.Logger = nil
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if body := <-bodies; body != "old payload" {
		t.Fatalf("initial body = %q", body)
	}
	if err := req.SetBody(nil); err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if body := <-bodies; body != "" {
		t.Fatalf("body after SetBody(nil) = %q, want empty", body)
	}
}
