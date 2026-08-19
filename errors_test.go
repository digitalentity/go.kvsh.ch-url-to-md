package urltomd

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

type dummyNetTimeoutError struct{}

func (e *dummyNetTimeoutError) Error() string   { return "i/o timeout" }
func (e *dummyNetTimeoutError) Timeout() bool   { return true }
func (e *dummyNetTimeoutError) Temporary() bool { return true }

func TestHTTPError_IsAndAs(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		target     error
		wantIs     bool
		statusCode int
		retryAfter time.Duration
	}{
		{
			name: "rate limited wrapped",
			err: &HTTPError{
				StatusCode: 429,
				Status:     "429 Too Many Requests",
				URL:        "https://example.com/api",
				RetryAfter: 10 * time.Second,
				Err:        ErrRateLimited,
			},
			target:     ErrRateLimited,
			wantIs:     true,
			statusCode: 429,
			retryAfter: 10 * time.Second,
		},
		{
			name: "challenge blocked wrapped",
			err: &HTTPError{
				StatusCode: 403,
				Status:     "403 Forbidden",
				URL:        "https://example.com/challenge",
				Err:        ErrChallengeBlocked,
			},
			target:     ErrChallengeBlocked,
			wantIs:     true,
			statusCode: 403,
		},
		{
			name: "forbidden wrapped",
			err: &HTTPError{
				StatusCode: 403,
				Status:     "403 Forbidden",
				URL:        "https://example.com/secret",
				Err:        ErrForbidden,
			},
			target:     ErrForbidden,
			wantIs:     true,
			statusCode: 403,
		},
		{
			name: "not found wrapped",
			err: &HTTPError{
				StatusCode: 404,
				Status:     "404 Not Found",
				URL:        "https://example.com/missing",
				Err:        ErrNotFound,
			},
			target:     ErrNotFound,
			wantIs:     true,
			statusCode: 404,
		},
		{
			name: "server unavailable wrapped",
			err: &HTTPError{
				StatusCode: 503,
				Status:     "503 Service Unavailable",
				URL:        "https://example.com/down",
				Err:        ErrServerUnavailable,
			},
			target:     ErrServerUnavailable,
			wantIs:     true,
			statusCode: 503,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.target) {
				t.Fatalf("errors.Is(%v, %v) = false, want %v", tt.err, tt.target, tt.wantIs)
			}

			var httpErr *HTTPError
			if !errors.As(tt.err, &httpErr) {
				t.Fatalf("errors.As(%v, &httpErr) = false", tt.err)
			}

			if httpErr.StatusCode != tt.statusCode {
				t.Errorf("got StatusCode %d, want %d", httpErr.StatusCode, tt.statusCode)
			}
			if httpErr.RetryAfter != tt.retryAfter {
				t.Errorf("got RetryAfter %v, want %v", httpErr.RetryAfter, tt.retryAfter)
			}
			if httpErr.Error() == "" {
				t.Error("expected non-empty Error() string")
			}
		})
	}
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "bare ErrChallengeBlocked",
			err:       ErrChallengeBlocked,
			retryable: false,
		},
		{
			name: "HTTPError wrapping ErrChallengeBlocked",
			err: &HTTPError{
				StatusCode: 429,
				Err:        ErrChallengeBlocked,
			},
			retryable: false,
		},
		{
			name:      "bare ErrRateLimited",
			err:       ErrRateLimited,
			retryable: true,
		},
		{
			name: "HTTPError 429",
			err: &HTTPError{
				StatusCode: 429,
				Err:        ErrRateLimited,
			},
			retryable: true,
		},
		{
			name: "HTTPError 502",
			err: &HTTPError{
				StatusCode: http.StatusBadGateway,
				Err:        ErrServerUnavailable,
			},
			retryable: true,
		},
		{
			name: "HTTPError 503",
			err: &HTTPError{
				StatusCode: http.StatusServiceUnavailable,
				Err:        ErrServerUnavailable,
			},
			retryable: true,
		},
		{
			name: "HTTPError 504",
			err: &HTTPError{
				StatusCode: http.StatusGatewayTimeout,
				Err:        ErrServerUnavailable,
			},
			retryable: true,
		},
		{
			name: "HTTPError 500",
			err: &HTTPError{
				StatusCode: http.StatusInternalServerError,
				Err:        ErrServerUnavailable,
			},
			retryable: true,
		},
		{
			name: "HTTPError 404",
			err: &HTTPError{
				StatusCode: http.StatusNotFound,
				Err:        ErrNotFound,
			},
			retryable: false,
		},
		{
			name: "HTTPError 403",
			err: &HTTPError{
				StatusCode: http.StatusForbidden,
				Err:        ErrForbidden,
			},
			retryable: false,
		},
		{
			name:      "net timeout error",
			err:       fmt.Errorf("network failed: %w", &dummyNetTimeoutError{}),
			retryable: true,
		},
		{
			name:      "generic error",
			err:       errors.New("unrelated error"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Retryable(tt.err)
			if got != tt.retryable {
				t.Errorf("Retryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

// Ensure dummyNetTimeoutError implements net.Error
var _ net.Error = (*dummyNetTimeoutError)(nil)
