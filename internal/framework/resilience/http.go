package resilience

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"healthcare/internal/framework/tenancy"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("integration circuit is open")

type circuit struct {
	failures int
	until    time.Time
}
type Client struct {
	HTTP     *http.Client
	Base     *url.URL
	mu       sync.Mutex
	circuits map[string]circuit
	slots    chan struct{}
}

// baseURL must come from the trusted tenant integration registry, never request input.
func New(baseURL string, concurrency int) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || concurrency < 1 {
		return nil, errors.New("invalid integration endpoint or concurrency")
	}
	return &Client{HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, Base: u, circuits: map[string]circuit{}, slots: make(chan struct{}, concurrency)}, nil
}

// Do retries only safe reads or operations carrying an explicit idempotency key.
// Credentials are provided by the caller's tenant-specific secret resolver.
func (c *Client) Do(ctx context.Context, method, path, idempotencyKey, bearer string, body []byte) ([]byte, int, error) {
	s, err := tenancy.From(ctx)
	if err != nil {
		return nil, 0, err
	}
	if len(body) > 1<<20 || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, 0, errors.New("invalid integration path or body")
	}
	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() || relative.Host != "" {
		return nil, 0, errors.New("invalid integration path")
	}
	c.mu.Lock()
	state := c.circuits[s.TenantID]
	c.mu.Unlock()
	if state.until.After(time.Now()) {
		return nil, 0, ErrCircuitOpen
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}
	attempts := 1
	if method == http.MethodGet || method == http.MethodHead || idempotencyKey != "" {
		attempts = 3
	}
	var data []byte
	var code int
	for attempt := 0; attempt < attempts; attempt++ {
		req, e := http.NewRequestWithContext(ctx, method, c.Base.ResolveReference(relative).String(), bytes.NewReader(body))
		if e != nil {
			return nil, 0, e
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Correlation-ID", s.CorrelationID)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		res, e := c.HTTP.Do(req)
		err = e
		code = 0
		if e == nil {
			code = res.StatusCode
			data, err = io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
			res.Body.Close()
			if len(data) > 1<<20 {
				return nil, code, errors.New("integration response exceeds limit")
			}
			if err == nil && code < 500 && code != 429 {
				c.record(s.TenantID, false)
				return data, code, nil
			}
			if err == nil {
				err = fmt.Errorf("integration returned status %d", code)
			}
		}
		if attempt+1 < attempts {
			delay := time.Duration(100*(1<<attempt)+rand.IntN(100)) * time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, code, ctx.Err()
			case <-timer.C:
			}
		}
	}
	c.record(s.TenantID, true)
	return nil, code, err
}
func (c *Client) record(tenant string, failed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !failed {
		delete(c.circuits, tenant)
		return
	}
	v := c.circuits[tenant]
	v.failures++
	if v.failures >= 3 {
		v.until = time.Now().Add(30 * time.Second)
	}
	c.circuits[tenant] = v
}
