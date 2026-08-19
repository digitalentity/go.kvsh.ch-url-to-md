# url-to-md

A Go library for fetching web pages or parsing HTML strings/readers and converting them to clean, readable Markdown. It uses `go-readability` for content extraction and `html-to-markdown` for conversion, while stripping common noise elements (ads, sidebars, newsletter boxes, cookie modals), detecting paywall truncation, and classifying anti-bot interstitials.

## Features

- **In-Memory & Network Extraction**: Parse remote URLs (`ConvertContext`, `Convert`) or in-memory HTML (`ConvertHTML`, `ConvertReader`).
- **Context Support**: Full `context.Context` cancellation and deadline propagation.
- **Paywall & Truncation Heuristics**: Advisory `Article.IsTruncated` boolean driven by JSON-LD (`isAccessibleForFree`), meta `robots: noarchive`, and language-gated teaser phrases.
- **Anti-Bot & Challenge Detection**: Structured typed error classification (`ErrChallengeBlocked`, `ErrRateLimited`, `ErrForbidden`, `ErrNotFound`, `ErrUnauthorized`, `ErrServerUnavailable`).
- **Retryability Helper**: `Retryable(err)` reports whether a failed operation is transient and safe to retry.
- **Noise Removal**: Automatically strips common web noise (ads, tracking, sidebars, newsletter banners, consent popups).
- **Flexible Options**: Custom `http.Client`, HTTP headers, timeouts, `http.CookieJar`, Zap logger, max body size limits, and title prepending.

## Installation

```bash
go get go.kvsh.ch/url-to-md
```

## Usage

### In-Memory HTML Conversion (Feed-First)

```go
package main

import (
	"fmt"
	"log"

	urltomd "go.kvsh.ch/url-to-md"
)

func main() {
	htmlContent := `<html><body><h1>Article Title</h1><p>Full body text...</p></body></html>`

	article, err := urltomd.ConvertHTML(htmlContent, "https://example.com/post")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Provenance: %s | Truncated: %v\n", article.Source, article.IsTruncated)
	fmt.Println(article.Content)
}
```

### Network Fetch with Context and Options

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"time"

	urltomd "go.kvsh.ch/url-to-md"
	"go.uber.org/zap"
)

func main() {
	jar, _ := cookiejar.New(nil)
	logger, _ := zap.NewDevelopment()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	article, err := urltomd.ConvertContext(ctx, "https://example.com/article",
		urltomd.WithTimeout(15*time.Second),
		urltomd.WithUserAgent("MyApp/1.0"),
		urltomd.WithHeader("Authorization", "Bearer token"),
		urltomd.WithCookieJar(jar),
		urltomd.WithLogger(logger),
		urltomd.WithTitle(true),
	)
	if err != nil {
		if errors.Is(err, urltomd.ErrChallengeBlocked) {
			log.Fatalf("Anti-bot challenge detected (retryable=%v): %v", urltomd.Retryable(err), err)
		}
		log.Fatalf("Convert failed (retryable=%v): %v", urltomd.Retryable(err), err)
	}

	fmt.Printf("Title: %s\nTruncated: %v\n\n%s\n", article.Title, article.IsTruncated, article.Content)
}
```

## Configuration Options

- `WithTimeout(d time.Duration)`: Sets HTTP client timeout (default: 30s).
- `WithUserAgent(ua string)`: Sets the User-Agent header (default: `url-to-md/1.0 (+https://go.kvsh.ch/url-to-md)`). The library identifies itself honestly rather than impersonating a browser, so operators can recognize and allowlist it.
- `WithHTTPClient(client *http.Client)`: Uses custom HTTP client for network requests. `WithTimeout` and `WithCookieJar` fill any fields left unset on the supplied client; the client itself is not mutated.
- `WithHeader(key, value string)`: Sets or overrides request headers.
- `WithMaxBodyBytes(limit int64)`: Sets maximum body read limit in bytes (default: 10MB).
- `WithTitle(include bool)`: Whether to prepend the article title as an H1 to Markdown content (default: true).
- `WithCookieJar(jar http.CookieJar)`: Provides cookie jar for HTTP client.
- `WithLogger(logger *zap.Logger)`: Provides Zap logger for internal operations (default: no-op).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
