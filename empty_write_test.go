package retryablehttp

import (
	"bytes"
	"net/http"
	"testing"
)

func TestRequestWriteToWithoutBody(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			req, err := NewRequest(method, "http://example.com", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("WriteTo panicked for a valid bodyless request: %v", recovered)
				}
			}()
			var output bytes.Buffer
			n, err := req.WriteTo(&output)
			if err != nil || n != 0 || output.Len() != 0 {
				t.Fatalf("WriteTo = %d, %v, %q; want an empty successful copy", n, err, output.String())
			}
		})
	}
}
