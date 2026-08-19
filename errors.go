package urltomd

import (
	"errors"
	"fmt"
	"net"
	"time"
)

var (
	ErrChallengeBlocked   = errors.New("anti-bot challenge or captcha detected")
	ErrRateLimited        = errors.New("rate limited by upstream server")
	ErrForbidden          = errors.New("access forbidden by upstream server")
	ErrNotFound           = errors.New("resource not found")
	ErrUnauthorized       = errors.New("unauthorized or authentication required")
	ErrServerUnavailable  = errors.New("upstream server error or unavailable")
	ErrClientError        = errors.New("upstream rejected the request")
	ErrInvalidContentType = errors.New("content type is not html")
	ErrEmptyContent       = errors.New("no extractable content")
)

// HTTPError carries the upstream response detail alongside a sentinel error so
// callers can both classify with errors.Is and act on the concrete status.
type HTTPError struct {
	StatusCode int
	Status     string
	URL        string
	RetryAfter time.Duration // Parsed from Retry-After; zero when absent.
	Err        error         // One of the sentinels above.
}

func (e *HTTPError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("http error %d (%s) for %s: %v (retry after %s)", e.StatusCode, e.Status, e.URL, e.Err, e.RetryAfter)
	}
	return fmt.Sprintf("http error %d (%s) for %s: %v", e.StatusCode, e.Status, e.URL, e.Err)
}

// Unwrap returns the underlying sentinel error for errors.Is and errors.As support.
func (e *HTTPError) Unwrap() error {
	return e.Err
}

// Retryable reports whether an error represents a transient failure suitable for
// retry. A challenge is an access decision, not a transient fault: it is always
// terminal, including when served with a status that would otherwise retry.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrChallengeBlocked) {
		return false
	}
	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrServerUnavailable) {
		return true
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
