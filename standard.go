package retryablehttp

import "net/http"

// NewStandardClient creates a new StandardClient with a Client with default settings.
func NewStandardClient() *StandardClient {
	return &StandardClient{NewClient()}
}

// StandardClient is a thin wrapper around Client to fulfill the Do function signature of http.Client
type StandardClient struct {
	*Client
}

// Do wraps calling an HTTP method with retries and fulfilling the Do function signature of http.Client
func (c *StandardClient) Do(req *http.Request) (*http.Response, error) {
	wrappedReq, err := FromRequest(req)
	if err != nil {
		return nil, err
	}

	return c.Client.Do(wrappedReq)
}
