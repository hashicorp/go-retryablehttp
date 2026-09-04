// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package retryablehttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

func normalizeError(err error) error {
	var dnsError *net.DNSError

	if errors.As(err, &dnsError) {
		// this field is populated with the DNS server on on CI, but not locally
		dnsError.Server = ""
	}

	return err
}

// redirectOnceError is returned by CheckRedirect so the inner http.Client.Do
// yields both a response (the redirect) and a *url.Error, which is the
// RoundTripper contract violation reported in issue #179.
func redirectOnceError(*http.Request, []*http.Request) error {
	return errors.New("stopped after 1 redirects")
}

func newRedirectingRetryClient(t *testing.T) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	t.Cleanup(ts.Close)

	retryClient := NewClient()
	retryClient.ErrorHandler = PassthroughErrorHandler
	retryClient.HTTPClient.CheckRedirect = redirectOnceError
	return retryClient, ts
}

func TestRoundTripper_RoundTrip_noResponseWithError(t *testing.T) {
	retryClient, ts := newRedirectingRetryClient(t)

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := (&RoundTripper{Client: retryClient}).RoundTrip(req)
	if resp != nil && err != nil {
		resp.Body.Close()
		t.Fatalf("RoundTrip returned both a non-nil response and error %v; http.RoundTripper forbids this", err)
	}
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatal("expected redirect error")
	}
}

func TestRoundTripper_StandardClient_noResponseAndErrorLog(t *testing.T) {
	retryClient, ts := newRedirectingRetryClient(t)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	resp, err := retryClient.StandardClient().Get(ts.URL)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected redirect error")
	}
	if got := buf.String(); strings.Contains(got, "RoundTripper returned a response & error") {
		t.Fatalf("net/http logged: %q", got)
	}
}
