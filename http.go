package retryablehttp

import "net/http"

// NewHTTPClient creates a new RetryingHTTPClient with a Client with default settings.
func NewHTTPClient() RetryingHTTPClient {
	return RetryingHTTPClient{NewClient()}
}

// RetryingHTTPClient is a thin wrapper around Client to fulfill the Do function signature of http.Client
type RetryingHTTPClient struct {
	*Client
}

// Do wraps calling an HTTP method with retries and fulfilling the Do function signature of http.Client
func (c *RetryingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	wrappedReq, err := FromRequest(req)
	if err != nil {
		return nil, err
	}

	return c.Client.Do(wrappedReq)
}
