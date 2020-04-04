package retryablehttp

import "net/http"

// NewCompatClient creates a new CompatClient with a Client with default settings.
func NewCompatClient() CompatClient {
	return CompatClient{NewClient()}
}

// CompatClient is a thin wrapper around Client to fulfill the Do function signature of http.Client
type CompatClient struct {
	*Client
}

// Do wraps calling an HTTP method with retries and fulfilling the Do function signature of http.Client
func (c *CompatClient) Do(req *http.Request) (*http.Response, error) {
	wrappedReq, err := FromRequest(req)
	if err != nil {
		return nil, err
	}

	return c.Client.Do(wrappedReq)
}
