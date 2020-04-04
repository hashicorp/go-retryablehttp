package retryablehttp

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetryingHTTPClient_Do(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("bad method: %s", r.Method)
		}

		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("bad body: %s", r.Body)
		}

		if string(bytes) != "Hello world" {
			t.Fatalf("bad body: %s", r.Body)
		}

		w.WriteHeader(200)
	}))
	defer server.Close()

	type fields struct {
		Client *Client
	}
	type args struct {
		req *http.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Happy path",
			fields: fields{
				Client: func() *Client {
					c := NewClient()
					c.HTTPClient = server.Client()
					return c
				}(),
			},
			args: args{
				req: func() *http.Request {
					request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("Hello world"))
					if err != nil {
						t.Fatalf("could create new request, %s", err)
					}

					return request
				}(),
			},
		},
		{
			name: "FromRequest errors",
			fields: fields{
				Client: func() *Client {
					c := NewClient()
					c.HTTPClient = server.Client()
					return c
				}(),
			},
			args: args{
				req: func() *http.Request {
					request, err := http.NewRequest(http.MethodPost, server.URL, ErrReader{})
					if err != nil {
						t.Fatalf("could create new request, %s", err)
					}

					return request
				}(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompatClient()
			c.Client = tt.fields.Client

			_, err := c.Do(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

type ErrReader struct{}

func (r ErrReader) Read(_ []byte) (int, error) {
	return 0, errors.New("an error")
}
