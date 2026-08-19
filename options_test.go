package urltomd

import (
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestOptions(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}

	customClient := &http.Client{Timeout: 5 * time.Second}
	logger := zap.NewExample()

	cfg := defaultConfig()
	opts := []Option{
		WithTimeout(15 * time.Second),
		WithUserAgent("CustomAgent/1.0"),
		WithTitle(false),
		WithCookieJar(jar),
		WithLogger(logger),
		WithHTTPClient(customClient),
		WithHeader("Authorization", "Bearer token123"),
		WithHeader("X-Custom", "value"),
		WithMaxBodyBytes(5 * 1024 * 1024),
	}

	for _, o := range opts {
		o(cfg)
	}

	if cfg.timeout != 15*time.Second {
		t.Errorf("timeout = %v, want 15s", cfg.timeout)
	}
	if cfg.userAgent != "CustomAgent/1.0" {
		t.Errorf("userAgent = %q, want %q", cfg.userAgent, "CustomAgent/1.0")
	}
	if cfg.includeTitle != false {
		t.Errorf("includeTitle = %v, want false", cfg.includeTitle)
	}
	if cfg.jar != jar {
		t.Errorf("jar = %v, want %v", cfg.jar, jar)
	}
	if cfg.logger != logger {
		t.Errorf("logger = %v, want %v", cfg.logger, logger)
	}
	if cfg.httpClient != customClient {
		t.Errorf("httpClient = %v, want %v", cfg.httpClient, customClient)
	}
	if cfg.headers.Get("Authorization") != "Bearer token123" {
		t.Errorf("Authorization header = %q, want %q", cfg.headers.Get("Authorization"), "Bearer token123")
	}
	if cfg.headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom header = %q, want %q", cfg.headers.Get("X-Custom"), "value")
	}
	if cfg.maxBodyBytes != 5*1024*1024 {
		t.Errorf("maxBodyBytes = %d, want %d", cfg.maxBodyBytes, 5*1024*1024)
	}
}
