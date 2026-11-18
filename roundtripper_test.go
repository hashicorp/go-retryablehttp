// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package retryablehttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRoundTripper_implements(t *testing.T) {
	// Compile-time proof of interface satisfaction.
	var _ http.RoundTripper = &RoundTripper{}
}

func TestRoundTripper_init(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	// Start with a new empty RoundTripper.
	rt := &RoundTripper{}

	// RoundTrip once.
	req, _ := http.NewRequest("GET", ts.URL, nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}

	// Check that the Client was initialized.
	if rt.Client == nil {
		t.Fatal("expected rt.Client to be initialized")
	}

	// Save the Client for later comparison.
	initialClient := rt.Client

	// RoundTrip again.
	req, _ = http.NewRequest("GET", ts.URL, nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}

	// Check that the underlying Client is unchanged.
	if rt.Client != initialClient {
		t.Fatalf("expected %v, got %v", initialClient, rt.Client)
	}
}

func TestRoundTripper_RoundTrip(t *testing.T) {
	var reqCount int32 = 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqNo := atomic.AddInt32(&reqCount, 1)
		if reqNo < 3 {
			w.WriteHeader(404)
		} else {
			w.WriteHeader(200)
			if _, err := w.Write([]byte("success!")); err != nil {
				t.Fatalf("failed to write: %v", err)
			}
		}
	}))
	defer ts.Close()

	// Make a client with some custom settings to verify they are used.
	retryClient := NewClient()
	retryClient.CheckRetry = func(_ context.Context, resp *http.Response, _ error) (bool, error) {
		return resp.StatusCode == 404, nil
	}

	// Get the standard client and execute the request.
	client := retryClient.StandardClient()
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Check the response to ensure the client behaved as expected.
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if v, err := io.ReadAll(resp.Body); err != nil {
		t.Fatal(err)
	} else if string(v) != "success!" {
		t.Fatalf("expected %q, got %q", "success!", v)
	}
}

func TestRoundTripper_TransportFailureErrorHandling(t *testing.T) {
	// Make a client with some custom settings to verify they are used.
	retryClient := NewClient()
	retryClient.CheckRetry = func(_ context.Context, resp *http.Response, err error) (bool, error) {
		if err != nil {
			return true, err
		}

		return false, nil
	}

	retryClient.ErrorHandler = PassthroughErrorHandler

	expectedError := &url.Error{
		Op:  "Get",
		URL: "http://999.999.999.999:999/",
		Err: &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &net.DNSError{
				Name:       "999.999.999.999",
				Err:        "no such host",
				IsNotFound: true,
			},
		},
	}

	// Get the standard client and execute the request.
	client := retryClient.StandardClient()
	_, err := client.Get("http://999.999.999.999:999/")

	// assert expectations
	if !reflect.DeepEqual(expectedError, normalizeError(err)) {
		t.Fatalf("expected %q, got %q", expectedError, err)
	}
}

func TestRoundTripper_RedirectError(t *testing.T) {
	redirectErr := errors.New("redirect rejected")
	for _, tc := range []struct {
		name        string
		redirectErr error
		wantErr     error
	}{
		{"rejected redirect", redirectErr, redirectErr},
		{"use last response", http.ErrUseLastResponse, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &roundTripperTestBody{Reader: strings.NewReader("redirect body")}
			retryClient := NewClient()
			retryClient.Logger = nil
			retryClient.RetryMax = 0
			retryClient.ErrorHandler = PassthroughErrorHandler
			retryClient.HTTPClient = &http.Client{
				Transport: roundTripperTestFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{"/next"}},
						Body:       body,
						Request:    req,
					}, nil
				}),
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return tc.redirectErr
				},
			}

			req, err := http.NewRequest(http.MethodGet, "http://example.test/start", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := retryClient.StandardClient().Transport.RoundTrip(req)
			if err != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr != nil {
				if resp != nil {
					t.Error("expected nil response on a rejected redirect")
				}
				if body.closeCount != 1 {
					t.Errorf("expected redirect body to be closed once, got %d", body.closeCount)
				}
				return
			}
			if resp == nil || resp.StatusCode != http.StatusFound {
				t.Fatalf("expected redirect response, got %v", resp)
			}
			defer resp.Body.Close()
			if body.closeCount != 0 {
				t.Errorf("expected redirect body to remain open, got %d closes", body.closeCount)
			}
			if b, err := io.ReadAll(resp.Body); err != nil || string(b) != "redirect body" {
				t.Fatalf("expected readable redirect body, got %q, %v", b, err)
			}
		})
	}
}

type roundTripperTestFunc func(*http.Request) (*http.Response, error)

func (f roundTripperTestFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type roundTripperTestBody struct {
	io.Reader
	closeCount int
}

func (b *roundTripperTestBody) Close() error {
	b.closeCount++
	return nil
}

func normalizeError(err error) error {
	var dnsError *net.DNSError

	if errors.As(err, &dnsError) {
		// this field is populated with the DNS server on on CI, but not locally
		dnsError.Server = ""
	}

	return err
}
