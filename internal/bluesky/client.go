package bluesky

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultHost = "https://public.api.bsky.app"

var ErrHTTP = errors.New("bluesky http error")

type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%v: %s returned %d", ErrHTTP, e.URL, e.StatusCode)
}

func (e *HTTPError) Unwrap() error { return ErrHTTP }

type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
	maxBody    int64
	retries    int
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = baseURL }
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) { c.httpClient = client }
}

func New(opts ...Option) *Client {
	c := &Client{
		baseURL: defaultHost,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		userAgent: "UnDolFan-BlueskyImporter/0.1 (+https://undolfan.mundodolphins.es)",
		maxBody:   8 << 20,
		retries:   2,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) GetAuthorFeed(ctx context.Context, actor, cursor string, limit int) (*FeedResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	v := url.Values{}
	v.Set("actor", actor)
	v.Set("limit", fmt.Sprint(limit))
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	var out FeedResponse
	if err := c.getJSON(ctx, "/xrpc/app.bsky.feed.getAuthorFeed", v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPostThread(ctx context.Context, uri string, depth int) (*ThreadResponse, error) {
	if depth <= 0 {
		depth = 100
	}
	v := url.Values{}
	v.Set("uri", uri)
	v.Set("depth", fmt.Sprint(depth))
	var out ThreadResponse
	if err := c.getJSON(ctx, "/xrpc/app.bsky.feed.getPostThread", v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, values url.Values, dest any) error {
	endpoint := c.baseURL + path + "?" + values.Encode()
	var last error
	for attempt := 0; attempt <= c.retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			last = err
		} else {
			err = decodeResponse(resp, c.maxBody, endpoint, dest)
			if err == nil {
				return nil
			}
			last = err
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || !transient(httpErr.StatusCode) {
				return err
			}
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 150 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

func decodeResponse(resp *http.Response, maxBody int64, endpoint string, dest any) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return &HTTPError{StatusCode: resp.StatusCode, URL: endpoint}
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxBody))
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode bluesky response: %w", err)
	}
	return nil
}

func transient(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout || code >= 500
}
