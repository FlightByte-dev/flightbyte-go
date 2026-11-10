package flightbyte

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// endpoint builds the URL for path under the base URL, each segment escaped.
func (c *Client) endpoint(path []string, query url.Values) string {
	var b strings.Builder
	b.WriteString(c.baseURL)
	for _, segment := range path {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(segment))
	}
	if len(query) > 0 {
		b.WriteByte('?')
		b.WriteString(query.Encode())
	}
	return b.String()
}

// do sends a request and decodes a JSON response into out, when out is not
// nil. Any 2xx status is a success.
func (c *Client) do(ctx context.Context, method string, path []string, query url.Values, body any, header http.Header, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	target := c.endpoint(path, query)
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("flightbyte: encoding the request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return fmt.Errorf("flightbyte: building %s %s: %w", method, target, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{Method: method, URL: target, Err: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ConnectionError{Method: method, URL: target, Err: err}
	}

	if rl := parseRateLimit(resp.Header); rl != nil {
		c.setRateLimit(rl)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newAPIError(resp.StatusCode, raw, resp.Header)
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("flightbyte: decoding the response of %s %s: %w", method, target, err)
	}
	return nil
}

// required reports a missing path parameter before it turns into a request
// for the wrong URL.
func required(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			return fmt.Errorf("flightbyte: %s is required", pairs[i])
		}
	}
	return nil
}

// setIf adds key to q when value is not empty.
func setIf(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}
