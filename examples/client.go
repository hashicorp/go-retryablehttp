package examples

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const baseURL = "http://localhost:9090"

func httpClientJitterUsage() {
	req, err := http.NewRequest("POST", baseURL, strings.NewReader("foo"))
	if err != nil {
		// handle error
	}

	retryReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		// handle error
	}

	client := retryablehttp.NewClient()
	client.Backoff = retryablehttp.LinearJitterBackoff
	client.RetryWaitMin = 800 * time.Millisecond
	client.RetryWaitMax = 1200 * time.Millisecond
	client.RetryMax = 4
	client.ErrorHandler = retryablehttp.PassthroughErrorHandler

	resp, err := client.Do(retryReq)
	if err != nil {
		// handle error
	}
	fmt.Println(resp, err)
}
