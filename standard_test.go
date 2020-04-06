package retryablehttp

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStandardClient_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Logf("bad method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Logf("bad body: %s, err: %s", r.Body, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if string(bytes) != "Hello world" {
			t.Logf("bad body: %s", r.Body)
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	tests := []struct {
		name    string
		req     *http.Request
		wantErr string
	}{
		{
			name: "Happy path",
			req: func() *http.Request {
				request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("Hello world"))
				if err != nil {
					t.Fatalf("unable to create request, %s", err)
				}

				return request
			}(),
		},
		{
			name: "FromRequest errors",
			req: func() *http.Request {
				request, err := http.NewRequest(http.MethodPost, server.URL, ErrReader{})
				if err != nil {
					t.Fatalf("unable to create request, %s", err)
				}

				return request
			}(),
			wantErr: "an error",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := NewStandardClient()

			got, err := c.Do(tt.req)
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("Do() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if got.StatusCode != http.StatusCreated {
				t.Fatalf("Do() statuscode = %d, want = %d", got.StatusCode, http.StatusCreated)
			}
		})
	}
}

type ErrReader struct{}

func (r ErrReader) Read(_ []byte) (int, error) {
	return 0, errors.New("an error")
}
