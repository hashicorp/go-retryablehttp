// The retryablehttp package provides a familiar HTTP client interface with
// automatic retries and exponential backoff. It is a thin wrapper over the
// standard net/http client library and exposes nearly the same public API.
// This makes retryablehttp very easy to drop into existing programs.
//
// retryablehttp performs automatic retries under certain conditions. Mainly, if
// an error is returned by the client (connection errors etc), or if a 500-range
// response is received, then a retry is invoked. Otherwise, the response is
// returned and left to the caller to interpret.
//
// The main difference from net/http is that requests which take a request body
// (POST/PUT et. al) require an io.ReadSeeker to be provided. This enables the
// request body to be "rewound" if the initial request fails so that the full
// request can be attempted again.
package retryablehttp

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
)

var (
	// Default retry configuration
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
	defaultRetryMax     = 4

	// defaultClient is used for performing requests without explicitly making
	// a new client. It is purposely private to avoid modifications.
	defaultClient = NewClient()

	// We need to consume response bodies to maintain http connections, but
	// limit the size we consume to respReadLimit.
	respReadLimit = int64(4096)
)

// LenReader is an interface implemented by many in-memory io.Reader's. Used
// for automatically sending the right Content-Length header when possible.
type LenReader interface {
	Len() int
}

// Request is a type alias for http.Request for backwards compatibility with
// previous versions of this library.
type Request = http.Request

// NewRequest creates a new HTTP request. It returns http.NewRequest and
// exists only for backwards compatibility.
func NewRequest(method, url string, body io.ReadSeeker) (*Request, error) {
	return http.NewRequest(method, url, body)
}

// RequestLogHook allows a function to run before each retry. The HTTP
// request which will be made, and the retry number (0 for the initial
// request) are available to users. The internal logger is exposed to
// consumers.
type RequestLogHook func(*log.Logger, *http.Request, int)

// ResponseLogHook is like RequestLogHook, but allows running a function
// on each HTTP response. This function will be invoked at the end of
// every HTTP request executed, regardless of whether a subsequent retry
// needs to be performed or not. If the response body is read or closed
// from this method, this will affect the response returned from Do().
type ResponseLogHook func(*log.Logger, *http.Response)

// CheckRetry specifies a policy for handling retries. It is called
// following each request with the response and error values returned by
// the http.Client. If CheckRetry returns false, the Client stops retrying
// and returns the response to the caller. If CheckRetry returns an error,
// that error value is returned in lieu of the error from the request. The
// Client will close any response body when retrying, but if the retry is
// aborted it is up to the CheckResponse callback to properly close any
// response body before returning.
type CheckRetry func(resp *http.Response, err error) (bool, error)

// Backoff specifies a policy for how long to wait between retries.
// It is called after a failing request to determine the amount of time
// that should pass before trying again.
type Backoff func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration

// Client is used to make HTTP requests. It adds additional functionality
// like automatic retries to tolerate minor outages.
type Client struct {
	HTTPClient *http.Client // Internal HTTP client.
	Logger     *log.Logger  // Customer logger instance.

	RetryWaitMin time.Duration // Minimum time to wait
	RetryWaitMax time.Duration // Maximum time to wait
	RetryMax     int           // Maximum number of retries

	// RequestLogHook allows a user-supplied function to be called
	// before each retry.
	RequestLogHook RequestLogHook

	// ResponseLogHook allows a user-supplied function to be called
	// with the response from each HTTP request executed.
	ResponseLogHook ResponseLogHook

	// CheckRetry specifies the policy for handling retries, and is called
	// after each request. The default policy is DefaultRetryPolicy.
	CheckRetry CheckRetry

	// Backoff specifies the policy for how long to wait between retries
	Backoff Backoff
}

// NewClient creates a new Client with default settings.
func NewClient() *Client {
	return &Client{
		HTTPClient:   cleanhttp.DefaultClient(),
		Logger:       log.New(os.Stderr, "", log.LstdFlags),
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   DefaultRetryPolicy,
		Backoff:      DefaultBackoff,
	}
}

// DefaultRetryPolicy provides a default callback for Client.CheckRetry, which
// will retry on connection errors and server errors.
func DefaultRetryPolicy(resp *http.Response, err error) (bool, error) {
	if err != nil {
		return true, err
	}
	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || resp.StatusCode >= 500 {
		return true, nil
	}

	return false, nil
}

// DefaultBackoff provides a default callback for Client.Backoff which
// will perform exponential backoff based on the attempt number and limited
// by the provided minimum and maximum durations.
func DefaultBackoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	mult := math.Pow(2, float64(attemptNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}

// nopCloser is like ioutil.NopCloser except it preserves Seek
type nopCloser struct {
	io.ReadSeeker
}

func (nopCloser) Close() error { return nil }

func (c nopCloser) Seek(offset int64, whence int) (int64, error) {
	return c.ReadSeeker.Seek(offset, whence)
}

// Do wraps calling an HTTP method with retries.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.Logger.Printf("[DEBUG] %s %s", req.Method, req.URL)

	if req.Body != nil {
		// if body is not rewindable then read it to a buffer that is rewindable
		if readSeeker, ok := req.Body.(io.ReadSeeker); ok {
			// always override any underlying Close function on req.Body so that
			// net/http does not close it after the first try
			req.Body = nopCloser{readSeeker}
		} else {
			buf, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			req.Body = nopCloser{bytes.NewReader(buf)}
		}
	}

	for i := 0; ; i++ {
		var code int // HTTP response code

		// Always rewind the request body when non-nil.
		if req.Body != nil {
			// this type cast will always succeed due to the steps above
			if _, err := req.Body.(io.Seeker).Seek(0, 0); err != nil {
				return nil, fmt.Errorf("failed to seek body: %v", err)
			}
		}

		if c.RequestLogHook != nil {
			c.RequestLogHook(c.Logger, req, i)
		}

		// Attempt the request
		resp, err := c.HTTPClient.Do(req)

		// Check if we should continue with retries.
		checkOK, checkErr := c.CheckRetry(resp, err)

		if err != nil {
			c.Logger.Printf("[ERR] %s %s request failed: %v", req.Method, req.URL, err)
		} else {
			// Call this here to maintain the behavior of logging all requests,
			// even if CheckRetry signals to stop.
			if c.ResponseLogHook != nil {
				// Call the response logger function if provided.
				c.ResponseLogHook(c.Logger, resp)
			}
		}

		// Now decide if we should continue.
		if !checkOK {
			if checkErr != nil {
				err = checkErr
			}
			return resp, err
		}

		// We're going to retry, consume any response to reuse the connection.
		if err == nil {
			c.drainBody(resp.Body)
		}

		remain := c.RetryMax - i
		if remain == 0 {
			break
		}
		wait := c.Backoff(c.RetryWaitMin, c.RetryWaitMax, i, resp)
		desc := fmt.Sprintf("%s %s", req.Method, req.URL)
		if code > 0 {
			desc = fmt.Sprintf("%s (status: %d)", desc, code)
		}
		c.Logger.Printf("[DEBUG] %s: retrying in %s (%d left)", desc, wait, remain)
		time.Sleep(wait)
	}

	// Return an error if we fall out of the retry loop
	return nil, fmt.Errorf("%s %s giving up after %d attempts",
		req.Method, req.URL, c.RetryMax+1)
}

// Try to read the response body so we can reuse this connection.
func (c *Client) drainBody(body io.ReadCloser) {
	defer body.Close()
	_, err := io.Copy(ioutil.Discard, io.LimitReader(body, respReadLimit))
	if err != nil {
		c.Logger.Printf("[ERR] error reading response body: %v", err)
	}
}

// Get is a shortcut for doing a GET request without making a new client.
func Get(url string) (*http.Response, error) {
	return defaultClient.Get(url)
}

// Get is a convenience helper for doing simple GET requests.
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Head is a shortcut for doing a HEAD request without making a new client.
func Head(url string) (*http.Response, error) {
	return defaultClient.Head(url)
}

// Head is a convenience method for doing simple HEAD requests.
func (c *Client) Head(url string) (*http.Response, error) {
	req, err := NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post is a shortcut for doing a POST request without making a new client.
func Post(url, bodyType string, body io.ReadSeeker) (*http.Response, error) {
	return defaultClient.Post(url, bodyType, body)
}

// Post is a convenience method for doing simple POST requests.
func (c *Client) Post(url, bodyType string, body io.ReadSeeker) (*http.Response, error) {
	req, err := NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", bodyType)
	return c.Do(req)
}

// PostForm is a shortcut to perform a POST with form data without creating
// a new client.
func PostForm(url string, data url.Values) (*http.Response, error) {
	return defaultClient.PostForm(url, data)
}

// PostForm is a convenience method for doing simple POST operations using
// pre-filled url.Values form data.
func (c *Client) PostForm(url string, data url.Values) (*http.Response, error) {
	return c.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}
