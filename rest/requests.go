package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RequestOptions struct {
	Method           string
	Headers          map[string]string
	Body             any
	WithNetworkRetry bool
	RetryDelay       time.Duration
}

type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
	Duration   time.Duration
}

var ErrRequestFailed = errors.New("HTTP(S)Req Failed")
var ErrResponseFailed = errors.New("Failed to gather response")

var reqClient = &http.Client{}

type RestClient struct {
	Client *http.Client
}

func (c *RestClient) Request(url string, opt *RequestOptions, ret any) (Response, error) {
	if opt == nil {
		opt = &RequestOptions{}
	}

	return Request(url, opt, ret, c.Client)
}

func Request(url string, opt *RequestOptions, ret any, client *http.Client) (Response, error) {
	method := "GET"
	headers := map[string]string{}
	if opt != nil {
		if opt.Method != "" {
			method = opt.Method
		}
		if opt.Headers != nil {
			headers = opt.Headers
		}
	}
	var response Response

	var body []byte
	if opt.Body != nil {
		bytes, err := json.Marshal(opt.Body)
		if err != nil {
			return response, fmt.Errorf("%w: failed to marshal request body: %w", ErrRequestFailed, err)
		}
		body = bytes
	}

	var req *http.Request
	if body != nil {
		obj, err := http.NewRequest(method, url, bytes.NewBuffer(body))
		if err != nil {
			return response, fmt.Errorf("%w: failed to create request (with body): %w", ErrRequestFailed, err)
		}
		obj.Header.Set("Content-Type", "application/json")
		req = obj
	} else {
		obj, err := http.NewRequest(method, url, nil)
		if err != nil {
			return response, fmt.Errorf("%w: failed to create request (without body): %w", ErrRequestFailed, err)
		}
		req = obj
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	useClient := reqClient
	if client != nil {
		useClient = client
	}

	var resp *http.Response
	for {
		var err error
		now := time.Now()
		resp, err = useClient.Do(req)
		response.Duration = time.Since(now)
		if err != nil {
			if opt == nil || !opt.WithNetworkRetry {
				return response, fmt.Errorf("%w: failed to send request: %w", ErrRequestFailed, err)
			}
			time.Sleep(opt.RetryDelay)
			continue
		}
		break
	}

	defer resp.Body.Close()

	response.StatusCode = resp.StatusCode
	response.Header = resp.Header.Clone()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, fmt.Errorf("%w: failed to read request: %w", ErrResponseFailed, err)
	}

	response.Body = bytes

	if ret != nil {
		if err := json.Unmarshal(bytes, ret); err != nil {
			return response, fmt.Errorf("%w: failed to parse JSON: %w. Body: %s", ErrResponseFailed, err, string(bytes))
		}
	}

	return response, nil
}
