package retryablehttp

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestBodyBytesClosesFactoryReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(path, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	var opened []*os.File
	t.Cleanup(func() {
		for _, file := range opened {
			file.Close()
		}
	})
	req, err := NewRequest(http.MethodPost, "http://example.com", ReaderFunc(func() (io.Reader, error) {
		file, err := os.Open(path)
		if err == nil {
			opened = append(opened, file)
		}
		return file, err
	}))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		body, err := req.BodyBytes()
		if err != nil || string(body) != "payload" {
			t.Fatalf("BodyBytes() = %q, %v", body, err)
		}
		if _, err := opened[len(opened)-1].Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("reader remains open after BodyBytes: %v", err)
		}
	}
}
