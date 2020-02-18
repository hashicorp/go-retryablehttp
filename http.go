package retryablehttp

import "net/http"

func NewHTTPClient() RetryingHTTPClient {
	return RetryingHTTPClient{NewClient()}
}

type RetryingHTTPClient struct {
	*Client
}

func (c *RetryingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	wrappedReq, err := FromRequest(req)
	if err != nil {
		return nil, err
	}

	return c.Client.Do(wrappedReq)
}
