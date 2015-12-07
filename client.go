package retryablehttp

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
)

var (
	// Default retry configuration
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 5 * time.Minute
	defaultRetryMax     = 32
)

// LenReader is an interface implemented by many in-memory io.Reader's. Used
// for automatically sending the right Content-Length header when possible.
type LenReader interface {
	Len() int
}

// Request wraps the metadata needed to create HTTP requests.
type Request struct {
	// body is a seekable reader over the request body payload. This is
	// used to rewind the request data in between retries.
	body io.ReadSeeker

	// Embed an HTTP request directly. This makes a *Request act exactly
	// like an *http.Request so that all meta methods are supported.
	*http.Request
}

// NewRequest creates a new wrapped request.
func NewRequest(method, url string, body io.ReadSeeker) (*Request, error) {
	// Wrap the body in a noop ReadCloser if non-nil. This prevents the
	// reader from being closed by the HTTP client.
	var rcBody io.ReadCloser
	if body != nil {
		rcBody = ioutil.NopCloser(body)
	}

	// Make the request with the noop-closer for the body.
	httpReq, err := http.NewRequest(method, url, rcBody)
	if err != nil {
		return nil, err
	}

	// Check if we can set the Content-Length automatically.
	if lr, ok := body.(LenReader); ok {
		httpReq.ContentLength = int64(lr.Len())
	}

	return &Request{body, httpReq}, nil
}

// Client is used to make HTTP requests. It adds additional functionality
// like automatic retries so that our internal services can tolerate minor
// outages.
type Client struct {
	HTTPClient *http.Client

	RetryWaitMin time.Duration // Minimum time to wait
	RetryWaitMax time.Duration // Maximum time to wait
	RetryMax     int           // Maximum number of retries
}

// NewClient creates a new Client. By default, HTTP this client has HTTP
// keepalives disabled.
func NewClient() *Client {
	// Create the HTTP client with keepalives disabled.
	client := cleanhttp.DefaultClient()
	client.Transport.(*http.Transport).DisableKeepAlives = true

	return &Client{
		HTTPClient:   client,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
	}
}

// Do wraps calling an HTTP method with retries.
func (c *Client) Do(req *Request) (*http.Response, error) {
	log.Printf("[DEBUG] %s %s", req.Method, req.URL)

	for i := 0; ; i++ {
		var code int // HTTP response code

		// Always rewind the request body when non-nil.
		if req.body != nil {
			if _, err := req.body.Seek(0, 0); err != nil {
				return nil, fmt.Errorf("failed to seek body: %v", err)
			}
		}

		// Attempt the request
		resp, err := c.HTTPClient.Do(req.Request)
		if err != nil {
			log.Printf("[ERR] %s %s request failed: %v", req.Method, req.URL, err)
			goto RETRY
		}
		code = resp.StatusCode

		// Check the response code. We retry on 500-range responses to allow
		// the server time to recover, as 500's are typically not permanent
		// errors and may relate to outages on the server side.
		if code%500 < 100 {
			resp.Body.Close()
			goto RETRY
		}
		return resp, nil

	RETRY:
		if i == c.RetryMax {
			break
		}
		wait := backoff(c.RetryWaitMin, c.RetryWaitMax, i)
		desc := fmt.Sprintf("%s %s", req.Method, req.URL)
		if code > 0 {
			desc = fmt.Sprintf("%s (status: %d)", desc, code)
		}
		log.Printf("[DEBUG] %s: retrying in %s", desc, wait)
		time.Sleep(wait)
	}

	// Return an error if we fall out of the retry loop
	return nil, fmt.Errorf("%s %s giving up after %d attempts",
		req.Method, req.URL, c.RetryMax+1)
}

// Get is a convenience helper for doing simple GET requests.
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// backoff is used to calculate how long to sleep before retrying
// after observing failures. It takes the minimum/maximum wait time and
// iteration, and returns the duration to wait.
func backoff(min, max time.Duration, iter int) time.Duration {
	mult := math.Pow(2, float64(iter)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}
