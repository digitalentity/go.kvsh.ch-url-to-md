package urltomd

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/go-shiori/go-readability"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	defaultTimeout   = 30 * time.Second
)

// Article holds the extracted content and metadata.
type Article struct {
	Title   string
	Byline  string
	Excerpt string
	Content string // Markdown
}

// Option is a functional option for configuring Convert.
type Option func(*config)

type config struct {
	timeout      time.Duration
	userAgent    string
	includeTitle bool
	jar          http.CookieJar
	logger       *zap.Logger
}

func defaultConfig() *config {
	return &config{
		timeout:      defaultTimeout,
		userAgent:    defaultUserAgent,
		includeTitle: true,
		logger:       zap.NewNop(),
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
	return func(c *config) { c.logger = logger }
}

// Convert fetches the page at rawURL, strips known noise elements, then
// extracts and returns the main article content as Markdown.
func Convert(rawURL string, opts ...Option) (*Article, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	cfg.logger.Info("converting url", zap.String("url", rawURL))

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	doc, err := fetch(rawURL, cfg)
	if err != nil {
		return nil, err
	}

	cleanDOM(doc)

	article, err := readability.FromDocument(doc, parsedURL)
	if err != nil {
		return nil, fmt.Errorf("extracting content: %w", err)
	}

	cfg.logger.Debug("extracted article", zap.String("title", article.Title))

	content, err := md.ConvertString(article.Content)
	if err != nil {
		return nil, fmt.Errorf("converting to markdown: %w", err)
	}

	if cfg.includeTitle && article.Title != "" {
		content = "# " + article.Title + "\n\n" + content
	}

	return &Article{
		Title:   article.Title,
		Byline:  article.Byline,
		Excerpt: article.Excerpt,
		Content: collapseBlankLines(content),
	}, nil
}

func fetch(rawURL string, cfg *config) (*html.Node, error) {
	cfg.logger.Debug("fetching url", zap.String("url", rawURL))

	client := &http.Client{
		Timeout: cfg.timeout,
		Jar:     cfg.jar,
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching url: %w", err)
	}
	defer resp.Body.Close()

	cfg.logger.Debug("fetched url", zap.String("url", rawURL), zap.Int("status", resp.StatusCode))

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return nil, fmt.Errorf("expected text/html, got %q", ct)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}
	return doc, nil
}

// collapseBlankLines reduces runs of more than one consecutive blank line to a
// single blank line and trims leading/trailing whitespace.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
