package urltomd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/go-shiori/go-readability"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

// Source represents the provenance of an article.
type Source string

const (
	// SourceFeed indicates the article was parsed from in-memory HTML without a network request.
	SourceFeed Source = "feed"
	// SourceFetched indicates the article was retrieved from the origin over HTTP.
	SourceFetched Source = "fetched"
)

// Article holds the extracted content and metadata.
type Article struct {
	Title         string
	Byline        string
	Excerpt       string
	Content       string // Markdown
	Language      string
	PublishedTime *time.Time
	IsTruncated   bool
	Source        Source
}

// Convert fetches the page at rawURL using context.Background() and returns extracted Markdown.
func Convert(rawURL string, opts ...Option) (*Article, error) {
	return ConvertContext(context.Background(), rawURL, opts...)
}

// ConvertContext fetches the page at rawURL respecting ctx cancellation and returns extracted Markdown.
func ConvertContext(ctx context.Context, rawURL string, opts ...Option) (*Article, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	cfg.logger.Info("converting url", zap.String("url", rawURL))

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	doc, err := fetch(ctx, rawURL, cfg)
	if err != nil {
		return nil, err
	}

	return processDOM(doc, parsedURL, SourceFetched, cfg)
}

// ConvertHTML parses, cleans, and extracts Markdown from an in-memory HTML string.
func ConvertHTML(htmlContent string, baseURL string, opts ...Option) (*Article, error) {
	return ConvertReader(strings.NewReader(htmlContent), baseURL, opts...)
}

// ConvertReader parses, cleans, and extracts Markdown from an io.Reader stream.
func ConvertReader(r io.Reader, baseURL string, opts ...Option) (*Article, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	limited := io.LimitReader(r, cfg.maxBodyBytes)
	doc, err := html.Parse(limited)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	// Challenge detection on in-memory document
	if err := detectChallengeDoc(doc); err != nil {
		return nil, err
	}

	targetURL, err := resolveBaseURL(doc, baseURL)
	if err != nil {
		return nil, err
	}

	return processDOM(doc, targetURL, SourceFeed, cfg)
}

func resolveBaseURL(doc *html.Node, rawBaseURL string) (*url.URL, error) {
	var explicitBase *url.URL
	if rawBaseURL != "" {
		u, err := url.Parse(rawBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid base url: %q", rawBaseURL)
		}
		explicitBase = u
	}

	// Document <base href> overrides the explicit base url for resolution, matching
	// browser behavior. A relative href is resolved against the explicit base; with
	// no explicit base to resolve against, it is unusable and ignored.
	if docBaseHref := extractDocBaseHref(doc); docBaseHref != "" {
		if docU, err := url.Parse(docBaseHref); err == nil {
			if docU.IsAbs() && docU.Host != "" {
				return docU, nil
			}
			if explicitBase != nil {
				return explicitBase.ResolveReference(docU), nil
			}
		}
	}

	if explicitBase != nil {
		return explicitBase, nil
	}

	return nil, nil
}

func extractDocBaseHref(doc *html.Node) string {
	var baseHref string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "base" {
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "href") {
					baseHref = a.Val
					return
				}
			}
		}
		for c := n.FirstChild; c != nil && baseHref == ""; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(baseHref)
}

func processDOM(doc *html.Node, parsedURL *url.URL, source Source, cfg *config) (*Article, error) {
	// 1. Sample document-level paywall signals before noise removal discards the
	// JSON-LD and meta nodes they live in. Evaluated in step 5 against real text.
	signals := collectPaywallSignals(doc)

	// 2. Clean noise elements
	cleanDOM(doc)

	// 3. Readability extraction
	article, err := readability.FromDocument(doc, parsedURL)
	if err != nil {
		return nil, fmt.Errorf("extracting content: %w", err)
	}

	if strings.TrimSpace(article.Content) == "" && strings.TrimSpace(article.TextContent) == "" {
		return nil, ErrEmptyContent
	}

	cfg.logger.Debug("extracted article", zap.String("title", article.Title))

	// 4. Markdown conversion
	content, err := md.ConvertString(article.Content)
	if err != nil {
		return nil, fmt.Errorf("converting to markdown: %w", err)
	}

	if cfg.includeTitle && article.Title != "" {
		content = "# " + article.Title + "\n\n" + content
	}

	collapsed := collapseBlankLines(content)
	if strings.TrimSpace(collapsed) == "" {
		return nil, ErrEmptyContent
	}

	// 5. Evaluate paywall signals against the extracted text.
	isTruncated := signals.truncated(article.TextContent, article.Language)

	return &Article{
		Title:         article.Title,
		Byline:        article.Byline,
		Excerpt:       strings.Trim(article.Excerpt, " \t\n\r"),
		Content:       collapsed,
		Language:      article.Language,
		PublishedTime: article.PublishedTime,
		IsTruncated:   isTruncated,
		Source:        source,
	}, nil
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
