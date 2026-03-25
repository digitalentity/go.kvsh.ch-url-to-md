# url-to-md

A Go library for fetching web pages and converting them to clean, readable Markdown. It uses `go-readability` for content extraction and `html-to-markdown` for conversion, while stripping common noise elements like ads, sidebars, and newsletter boxes.

## Features

- **Content Extraction**: Extracts the main article content using readability algorithms.
- **Markdown Conversion**: Converts HTML to high-quality Markdown.
- **Noise Removal**: Automatically strips common web noise (ads, tracking, sidebars, newsletters).
- **Customizable**:
  - Configure timeouts and User-Agent.
  - Optional cookie handling with `http.CookieJar` (useful for certain redirects).
  - Integration with `uber-go/zap` for structured logging.
  - Option to include/exclude the article title as an H1.

## Installation

```bash
go get go.kvsh.ch/url-to-md
```

## Usage

### Basic Example

```go
package main

import (
	"fmt"
	"log"
	urltomd "go.kvsh.ch/url-to-md"
)

func main() {
	article, err := urltomd.Convert("https://example.com/article")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("# %s\n\n%s", article.Title, article.Content)
}
```

### Advanced Usage with Options

```go
package main

import (
	"fmt"
	"log"
	"net/http/cookiejar"
	"time"

	urltomd "go.kvsh.ch/url-to-md"
	"go.uber.org/zap"
)

func main() {
	// Optional: Set up a cookie jar for sites that require it for redirects
	jar, _ := cookiejar.New(nil)

	// Optional: Set up a logger
	logger, _ := zap.NewDevelopment()

	article, err := urltomd.Convert("https://example.com/article",
		urltomd.WithTimeout(15*time.Second),
		urltomd.WithUserAgent("MyApp/1.0"),
		urltomd.WithCookieJar(jar),
		urltomd.WithLogger(logger),
		urltomd.WithTitle(true), // Include title as H1 in Content
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(article.Content)
}
```

## Configuration Options

- `WithTimeout(d time.Duration)`: Sets the HTTP request timeout (default: 30s).
- `WithUserAgent(ua string)`: Sets the User-Agent header for the request.
- `WithTitle(include bool)`: Whether to prepend the article title as an H1 to the Markdown content (default: true).
- `WithCookieJar(jar http.CookieJar)`: Provides a cookie jar for the HTTP client.
- `WithLogger(logger *zap.Logger)`: Provides a Zap logger for internal operations (default: no-op).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
