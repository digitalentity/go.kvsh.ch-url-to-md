package urltomd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/html"
)

func fetch(ctx context.Context, rawURL string, cfg *config) (*html.Node, error) {
	cfg.logger.Debug("fetching url", zap.String("url", rawURL))

	client := clientFor(cfg)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("User-Agent", cfg.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	for k, vv := range cfg.headers {
		req.Header.Del(k)
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("fetching url: %w", err)
	}
	defer resp.Body.Close()

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

	// If status is not 200, attempt to detect challenge first, then map status to sentinel
	if resp.StatusCode != http.StatusOK {
		var doc *html.Node
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml+xml") {
			r := io.LimitReader(resp.Body, cfg.maxBodyBytes)
			doc, _ = html.Parse(r)
		}

		if err := detectChallengeResponse(resp, doc); err != nil {
			return nil, &HTTPError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				URL:        rawURL,
				RetryAfter: retryAfter,
				Err:        err,
			}
		}

		sentinel := mapStatusCodeToSentinel(resp.StatusCode)
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        rawURL,
			RetryAfter: retryAfter,
			Err:        sentinel,
		}
	}

	cfg.logger.Debug("fetched url", zap.String("url", rawURL), zap.Int("status", resp.StatusCode))

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml+xml") {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        rawURL,
			Err:        ErrInvalidContentType,
		}
	}

	reader := io.LimitReader(resp.Body, cfg.maxBodyBytes)
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	// Check for challenge pages that return 200 OK (e.g. interstitial HTML titles)
	if err := detectChallengeResponse(resp, doc); err != nil {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        rawURL,
			RetryAfter: retryAfter,
			Err:        err,
		}
	}

	return doc, nil
}

// clientFor returns the HTTP client to use. A caller-supplied client is honored
// as-is, except that WithTimeout and WithCookieJar fill fields the caller left
// unset rather than being silently discarded. The caller's client is never
// mutated; a shallow copy carries the adjustments.
func clientFor(cfg *config) *http.Client {
	if cfg.httpClient == nil {
		return &http.Client{Timeout: cfg.timeout, Jar: cfg.jar}
	}

	needsTimeout := cfg.httpClient.Timeout == 0 && cfg.timeout > 0
	needsJar := cfg.httpClient.Jar == nil && cfg.jar != nil
	if !needsTimeout && !needsJar {
		return cfg.httpClient
	}

	adjusted := *cfg.httpClient
	if needsTimeout {
		adjusted.Timeout = cfg.timeout
	}
	if needsJar {
		adjusted.Jar = cfg.jar
	}
	return &adjusted
}

func mapStatusCodeToSentinel(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusProxyAuthRequired:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound, http.StatusGone:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		if status >= 400 && status < 500 {
			return ErrClientError
		}
		return ErrServerUnavailable
	}
}

func parseRetryAfter(val string) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(val); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	formats := []string{
		http.TimeFormat,
		time.RFC850,
		time.ANSIC,
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, val); err == nil {
			d := time.Until(t)
			if d > 0 {
				return d
			}
			return 0
		}
	}
	return 0
}
