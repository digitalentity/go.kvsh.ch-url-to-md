package urltomd

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	// defaultUserAgent identifies the library honestly so operators can recognize,
	// rate-limit, or allowlist it. Callers override with WithUserAgent.
	defaultUserAgent    = "url-to-md/1.0 (+https://go.kvsh.ch/url-to-md)"
	defaultTimeout      = 30 * time.Second
	defaultMaxBodyBytes = 10 * 1024 * 1024 // 10 MB
)

// Option is a functional option for configuring conversion behavior.
type Option func(*config)

type config struct {
	timeout      time.Duration
	userAgent    string
	includeTitle bool
	jar          http.CookieJar
	logger       *zap.Logger
	httpClient   *http.Client
	headers      http.Header
	maxBodyBytes int64
}

func defaultConfig() *config {
	return &config{
		timeout:      defaultTimeout,
		userAgent:    defaultUserAgent,
		includeTitle: true,
		logger:       zap.NewNop(),
		headers:      make(http.Header),
		maxBodyBytes: defaultMaxBodyBytes,
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithUserAgent sets the User-Agent header sent with the request.
func WithUserAgent(ua string) Option {
	return func(c *config) { c.userAgent = ua }
}

// WithTitle controls whether a Markdown H1 title is prepended to the content.
func WithTitle(include bool) Option {
	return func(c *config) { c.includeTitle = include }
}

// WithCookieJar sets the CookieJar for the HTTP client.
func WithCookieJar(jar http.CookieJar) Option {
	return func(c *config) { c.jar = jar }
}

// WithLogger sets the zap logger for logging.
func WithLogger(logger *zap.Logger) Option {
	return func(c *config) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithHTTPClient sets a custom http.Client to use for network requests.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) { c.httpClient = client }
}

// WithHeader sets or overwrites a request header key with value.
func WithHeader(key, value string) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = make(http.Header)
		}
		c.headers.Set(key, value)
	}
}

// WithMaxBodyBytes sets the maximum number of bytes to read from a response or reader stream.
func WithMaxBodyBytes(limit int64) Option {
	return func(c *config) {
		if limit > 0 {
			c.maxBodyBytes = limit
		}
	}
}
