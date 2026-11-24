// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package retryablehttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRequestStringReaderOffset(t *testing.T) {
	var received []string
	var receivedMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		receivedMu.Lock()
		received = append(received, string(body))
		attempt := len(received)
		receivedMu.Unlock()
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	body := strings.NewReader("prefix:payload")
	if _, err := body.Seek(int64(len("prefix:")), io.SeekStart); err != nil {
		t.Fatal(err)
	}
	req, err := NewRequest(http.MethodPost, server.URL, body)
	if err != nil {
		t.Fatal(err)
	}
	if req.ContentLength != int64(len("payload")) {
		t.Fatalf("ContentLength = %d", req.ContentLength)
	}
	client := NewClient()
	client.Logger = nil
	client.RetryMax = 1
	client.RetryWaitMin = 0
	client.RetryWaitMax = 0
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	receivedMu.Lock()
	defer receivedMu.Unlock()
	if len(received) != 2 || received[0] != "payload" || received[1] != "payload" {
		t.Fatalf("received bodies = %q", received)
	}
	if offset, err := body.Seek(0, io.SeekCurrent); err != nil || offset != int64(len("prefix:")) {
		t.Fatalf("source offset = %d, error = %v", offset, err)
	}
}
