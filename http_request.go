package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HttpRequest
// Little helper to make some http requests
// just set the method, url, headers and query parameters of the request
type HttpRequest struct {
	Method  string
	Url     string
	Headers map[string]string
	Query   map[string]string
}

// Do
// Execute the http request
// Set Authorization header if provided in the context
// Then based on the returned status code
// 200: unmarshal the response.Body into the `body` argument
// any: read and returns the response.Body as an error
func (request *HttpRequest) Do(ctx context.Context, body any) (*http.Response, error) {
	req, err := http.NewRequest(request.Method, request.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest failed: %w", err)
	}

	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	if request.Query != nil {
		params := req.URL.Query()

		for key, value := range request.Query {
			params.Add(key, value)
		}

		req.URL.RawQuery = params.Encode()
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return res, fmt.Errorf("http.DefaultClient.Do failed: %w", err)
	}

	switch res.StatusCode {
	case 200:
		if err := json.NewDecoder(res.Body).Decode(body); err != nil {
			_, _ = io.ReadAll(res.Body)

			return res, fmt.Errorf("json.NewDecoder failed: %w", err)
		}
	default:
		bytes, _ := io.ReadAll(res.Body)

		return res, fmt.Errorf("request %s %s?%s failed: %s.", request.Method, request.Url, req.URL.RawQuery, string(bytes))
	}

	return res, nil
}
