package netx

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var defaultTransport = &http.Transport{
	Proxy:               http.ProxyFromEnvironment,
	DisableKeepAlives:   true,
	TLSHandshakeTimeout: 10 * time.Second,
}
var _ = defaultTransport

type Client struct {
	httpClient *http.Client
	userAgent  string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout:   20 * time.Second,
			Transport: defaultTransport,
		},
		userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	}
}

func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Referer", "https://quote.eastmoney.com/center/gridlist.html")
		req.Header.Set("Origin", "https://quote.eastmoney.com")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
		req.Header.Set("Connection", "close")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			// brief backoff on network errors
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
			return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
		}

		body := resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil, err
			}
			defer gr.Close()
			body = gr
		}
		return io.ReadAll(body)
	}
	return nil, lastErr
}
