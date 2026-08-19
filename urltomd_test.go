package urltomd_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	urltomd "go.kvsh.ch/url-to-md"
)

// articleHTML is a realistic article page with surrounding noise that the
// cleaner and readability should strip (nav, aside, footer).
const articleHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Go Concurrency Patterns</title></head>
<body>
  <nav id="navigation">
    <a href="/">Home</a> | <a href="/about">About</a> | <a href="/contact">Contact</a>
  </nav>
  <aside class="sidebar">
    <h3>Advertisement</h3>
    <p>Buy our product now!</p>
  </aside>
  <article>
    <h1>Go Concurrency Patterns</h1>
    <p class="byline">By Jane Doe</p>
    <p>Go makes it easy to write concurrent programs using goroutines and channels.
    A goroutine is a lightweight thread managed by the Go runtime. You can start a
    goroutine simply by using the <code>go</code> keyword before a function call.</p>
    <p>Channels provide a way for goroutines to communicate with each other and
    synchronize their execution. By default, sends and receives block until the
    other side is ready, allowing goroutines to synchronize without explicit locks.</p>
    <p>The select statement lets a goroutine wait on multiple communication
    operations. It blocks until one of its cases can run, then executes that case.
    It chooses one at random if multiple are ready at the same time.</p>
    <p>Context propagation is another important pattern, used for cancellation,
    deadlines, and passing request-scoped values across API boundaries and between
    goroutines.</p>
  </article>
  <footer id="footer">
    <p>Copyright 2024 Example Corp. All rights reserved.</p>
  </footer>
</body>
</html>`

// noisyHTML exercises class-, id-, and role-based noise removal.
const noisyHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Noisy Page</title></head>
<body>
  <div class="cookie-banner">Accept our cookies!</div>
  <div id="newsletter">Subscribe to our newsletter</div>
  <div role="navigation"><a href="/">Home</a></div>
  <div class="sponsored-content">Sponsored: Buy this product</div>
  <article>
    <h1>The Real Article</h1>
    <p>Goroutines are the fundamental unit of concurrency in Go. They are
    lightweight and managed by the Go runtime scheduler, which multiplexes them
    onto OS threads efficiently.</p>
    <p>Unlike OS threads, goroutines have a small initial stack that can grow
    and shrink as needed. This makes it practical to spawn thousands or even
    millions of goroutines in a single program.</p>
  </article>
  <div class="related-articles">Read more: related links here</div>
</body>
</html>`

func serve(t *testing.T, body, contentType string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
}

func TestConvert_ExtractsArticleContent(t *testing.T) {
	srv := serve(t, articleHTML, "text/html; charset=utf-8")
	defer srv.Close()

	article, err := urltomd.Convert(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if article.Title == "" {
		t.Error("title should not be empty")
	}
	if !strings.Contains(article.Content, "goroutine") {
		t.Errorf("content missing article text, got:\n%s", article.Content)
	}
	if article.Source != urltomd.SourceFetched {
		t.Errorf("article.Source = %v, want %v", article.Source, urltomd.SourceFetched)
	}
}

func TestConvert_StripsSemanticNoise(t *testing.T) {
	srv := serve(t, articleHTML, "text/html; charset=utf-8")
	defer srv.Close()

	article, err := urltomd.Convert(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, unwanted := range []string{"Advertisement", "Copyright 2024"} {
		if strings.Contains(article.Content, unwanted) {
			t.Errorf("content should not contain %q, got:\n%s", unwanted, article.Content)
		}
	}
}

func TestConvert_StripsNoiseByClassAndRole(t *testing.T) {
	srv := serve(t, noisyHTML, "text/html; charset=utf-8")
	defer srv.Close()

	article, err := urltomd.Convert(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, unwanted := range []string{"Accept our cookies", "Subscribe to our newsletter", "Sponsored: Buy", "related links"} {
		if strings.Contains(article.Content, unwanted) {
			t.Errorf("content should not contain %q:\n%s", unwanted, article.Content)
		}
	}
	if !strings.Contains(article.Content, "Goroutines") {
		t.Errorf("article content should be preserved, got:\n%s", article.Content)
	}
}

func TestConvert_WithTitle(t *testing.T) {
	t.Run("included", func(t *testing.T) {
		srv := serve(t, articleHTML, "text/html; charset=utf-8")
		defer srv.Close()

		article, err := urltomd.Convert(srv.URL, urltomd.WithTitle(true))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(article.Content, "# ") {
			t.Errorf("expected content to start with H1, got: %q", article.Content[:min(80, len(article.Content))])
		}
	})

	t.Run("suppressed", func(t *testing.T) {
		srv := serve(t, articleHTML, "text/html; charset=utf-8")
		defer srv.Close()

		article, err := urltomd.Convert(srv.URL, urltomd.WithTitle(false))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.HasPrefix(article.Content, "# Go Concurrency") {
			t.Errorf("expected title to be suppressed, got: %q", article.Content[:min(80, len(article.Content))])
		}
	})
}

func TestConvert_WithUserAgent(t *testing.T) {
	const wantUA = "TestAgent/9000"
	var gotUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	if _, err := urltomd.Convert(srv.URL, urltomd.WithUserAgent(wantUA)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUA != wantUA {
		t.Errorf("User-Agent: got %q, want %q", gotUA, wantUA)
	}
}

func TestConvertContext_Headers(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	_, err := urltomd.ConvertContext(context.Background(), srv.URL, urltomd.WithHeader("Authorization", "Bearer secret-token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer secret-token")
	}
}

func TestConvertContext_Cancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := urltomd.ConvertContext(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}

	// Must be context cancellation, not wrapped into *HTTPError
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.DeadlineExceeded/Canceled, got %v", err)
	}
	var httpErr *urltomd.HTTPError
	if errors.As(err, &httpErr) {
		t.Errorf("cancellation should not be wrapped into *HTTPError")
	}
}

func TestConvert_InvalidURL(t *testing.T) {
	_, err := urltomd.Convert("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestConvertContext_StatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		header     map[string]string
		body       string
		wantTarget error
	}{
		{
			name:       "404 Not Found",
			status:     http.StatusNotFound,
			body:       "<html><body>Not found</body></html>",
			wantTarget: urltomd.ErrNotFound,
		},
		{
			name:       "401 Unauthorized",
			status:     http.StatusUnauthorized,
			body:       "<html><body>Login required</body></html>",
			wantTarget: urltomd.ErrUnauthorized,
		},
		{
			name:       "403 Forbidden without challenge",
			status:     http.StatusForbidden,
			body:       "<html><body>Forbidden</body></html>",
			wantTarget: urltomd.ErrForbidden,
		},
		{
			name:       "429 Rate Limited",
			status:     http.StatusTooManyRequests,
			header:     map[string]string{"Retry-After": "30"},
			body:       "<html><body>Slow down</body></html>",
			wantTarget: urltomd.ErrRateLimited,
		},
		{
			name:       "503 Service Unavailable",
			status:     http.StatusServiceUnavailable,
			body:       "<html><body>Server busy</body></html>",
			wantTarget: urltomd.ErrServerUnavailable,
		},
		{
			name:       "403 with Cloudflare challenge signature",
			status:     http.StatusForbidden,
			header:     map[string]string{"Cf-Ray": "987654"},
			body:       "<html><body>Blocked</body></html>",
			wantTarget: urltomd.ErrChallengeBlocked,
		},
		{
			name:       "200 with Cloudflare challenge title",
			status:     http.StatusOK,
			body:       "<html><head><title>Just a moment...</title></head><body>Checking browser</body></html>",
			wantTarget: urltomd.ErrChallengeBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := urltomd.ConvertContext(context.Background(), srv.URL)
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tt.status)
			}

			if !errors.Is(err, tt.wantTarget) {
				t.Errorf("errors.Is(err, %v) = false, got %v", tt.wantTarget, err)
			}

			var httpErr *urltomd.HTTPError
			if !errors.As(err, &httpErr) {
				t.Errorf("expected *HTTPError wrapper, got %T: %v", err, err)
			} else if httpErr.StatusCode != tt.status {
				t.Errorf("httpErr.StatusCode = %d, want %d", httpErr.StatusCode, tt.status)
			}
		})
	}
}

func TestConvert_NonHTMLContentType(t *testing.T) {
	srv := serve(t, `{"key":"value"}`, "application/json")
	defer srv.Close()

	_, err := urltomd.Convert(srv.URL)
	if err == nil {
		t.Fatal("expected error for non-HTML content type, got nil")
	}
	if !errors.Is(err, urltomd.ErrInvalidContentType) {
		t.Errorf("expected ErrInvalidContentType, got %v", err)
	}
}

func TestConvert_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	_, err := urltomd.Convert(srv.URL, urltomd.WithTimeout(50*time.Millisecond))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestConvertHTML_And_ConvertReader(t *testing.T) {
	t.Run("ConvertHTML standard", func(t *testing.T) {
		article, err := urltomd.ConvertHTML(articleHTML, "https://example.com/blog/post-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if article.Title != "Go Concurrency Patterns" {
			t.Errorf("title = %q, want %q", article.Title, "Go Concurrency Patterns")
		}
		if article.Source != urltomd.SourceFeed {
			t.Errorf("article.Source = %v, want %v", article.Source, urltomd.SourceFeed)
		}
	})

	t.Run("ConvertReader challenge document", func(t *testing.T) {
		challengeHTML := `<html><head><title>Attention Required! | Cloudflare</title></head><body>Captcha</body></html>`
		_, err := urltomd.ConvertReader(strings.NewReader(challengeHTML), "")
		if err == nil {
			t.Fatal("expected ErrChallengeBlocked, got nil")
		}
		if !errors.Is(err, urltomd.ErrChallengeBlocked) {
			t.Errorf("expected ErrChallengeBlocked, got %v", err)
		}
	})

	t.Run("ConvertReader empty content", func(t *testing.T) {
		emptyHTML := `<html><head><title>Empty</title></head><body></body></html>`
		_, err := urltomd.ConvertReader(strings.NewReader(emptyHTML), "")
		if err == nil {
			t.Fatal("expected ErrEmptyContent, got nil")
		}
		if !errors.Is(err, urltomd.ErrEmptyContent) {
			t.Errorf("expected ErrEmptyContent, got %v", err)
		}
	})
}

func TestConvertHTML_InvalidBaseURL(t *testing.T) {
	// A malformed baseURL is a caller bug. Degrading silently to unresolved links
	// would hide it, so conversion fails instead.
	for _, base := range []string{"invalid-url-without-scheme", "://missing-scheme", "https://"} {
		if _, err := urltomd.ConvertHTML(articleHTML, base); err == nil {
			t.Errorf("baseURL %q: expected error, got nil", base)
		}
	}
}

func TestConvertHTML_EmptyBaseURL(t *testing.T) {
	// An empty baseURL is valid: relative links are emitted unresolved rather than
	// failing the conversion, because feed content often carries no reliable base.
	article, err := urltomd.ConvertHTML(articleHTML, "")
	if err != nil {
		t.Fatalf("unexpected error with empty baseURL: %v", err)
	}
	if article.Title == "" {
		t.Error("expected non-empty article title")
	}
}

func TestSourceProvenance(t *testing.T) {
	t.Run("network path reports fetched", func(t *testing.T) {
		srv := serve(t, articleHTML, "text/html; charset=utf-8")
		defer srv.Close()

		article, err := urltomd.Convert(srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if article.Source != urltomd.SourceFetched {
			t.Errorf("article.Source = %v, want %v", article.Source, urltomd.SourceFetched)
		}
	})

	t.Run("in-memory path reports feed", func(t *testing.T) {
		article, err := urltomd.ConvertHTML(articleHTML, "https://example.com/blog/post-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if article.Source != urltomd.SourceFeed {
			t.Errorf("article.Source = %v, want %v", article.Source, urltomd.SourceFeed)
		}
	})

	t.Run("reader path reports feed", func(t *testing.T) {
		article, err := urltomd.ConvertReader(strings.NewReader(articleHTML), "https://example.com/blog/post-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if article.Source != urltomd.SourceFeed {
			t.Errorf("article.Source = %v, want %v", article.Source, urltomd.SourceFeed)
		}
	})
}

func TestConvertReader_MaxBodyBytes(t *testing.T) {
	// The limit must actually bound the read, not merely be stored in the config:
	// it is the guard against a hostile or runaway stream exhausting memory.
	// The marker sits inside <article> so readability keeps it, and the filler
	// guarantees it falls past a limit set to half the document.
	const head = `<!DOCTYPE html><html lang="en"><head><title>Limit Test</title></head>` +
		`<body><article><h1>Limit Test</h1>`
	filler := strings.Repeat(
		`<p>Filler sentence padding the document body well past any small byte limit.</p>`, 40)
	// Real prose with commas: readability discards short, comma-less paragraphs as
	// boilerplate, which would drop the marker for reasons unrelated to the limit.
	const tail = `<p>TailMarkerPastLimit, this closing paragraph is deliberately ` +
		`long enough, and punctuated with commas, that readability retains it as ` +
		`genuine article prose rather than discarding it as boilerplate.</p>` +
		`</article></body></html>`

	doc := head + filler + tail
	limit := int64(len(head) + len(filler)/2)

	t.Run("positive control: marker is visible without a limit", func(t *testing.T) {
		// Guards the subtests below from passing vacuously: if the marker were
		// dropped by extraction rather than by the byte limit, this fails.
		article, err := urltomd.ConvertReader(strings.NewReader(doc), "https://example.com/post")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(article.Content, "TailMarkerPastLimit") {
			t.Fatalf("marker absent even unbounded; the limit subtests prove nothing\ngot:\n%s", article.Content)
		}
	})

	t.Run("truncates an oversized reader", func(t *testing.T) {
		article, err := urltomd.ConvertReader(
			strings.NewReader(doc),
			"https://example.com/post",
			urltomd.WithMaxBodyBytes(limit),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(article.Content, "TailMarkerPastLimit") {
			t.Error("content past maxBodyBytes was read; limit not enforced")
		}
	})

	t.Run("limit applies to the network path", func(t *testing.T) {
		srv := serve(t, doc, "text/html; charset=utf-8")
		defer srv.Close()

		article, err := urltomd.Convert(srv.URL, urltomd.WithMaxBodyBytes(limit))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(article.Content, "TailMarkerPastLimit") {
			t.Error("content past maxBodyBytes was read on the network path")
		}
	})

	t.Run("reads whole body when under the limit", func(t *testing.T) {
		article, err := urltomd.ConvertReader(
			strings.NewReader(doc),
			"https://example.com/post",
			urltomd.WithMaxBodyBytes(int64(len(doc))+1024),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(article.Content, "TailMarkerPastLimit") {
			t.Errorf("content truncated below the limit, got:\n%s", article.Content)
		}
	})
}

func TestConvertHTML_DocumentBaseHrefWins(t *testing.T) {
	const relativeLinkHTML = `<!DOCTYPE html>
<html lang="en">
<head><title>Base Href Test</title>%s</head>
<body>
  <article>
    <h1>Base Href Test</h1>
    <p>Link resolution should follow the document base when one is present.
    This paragraph exists only to give readability enough text to treat the
    article element as the primary content of the page rather than boilerplate.
    See <a href="relative/page">the relative link</a> for the resolved target.</p>
    <p>Additional filler text keeps the extracted body comfortably above the
    minimum length that the readability heuristics expect from a real article.</p>
  </article>
</body>
</html>`

	tests := []struct {
		name    string
		baseTag string
		baseURL string
		want    string
	}{
		{
			name:    "absolute document base overrides argument",
			baseTag: `<base href="https://docbase.example.org/docs/">`,
			baseURL: "https://argument.example.com/blog/",
			want:    "https://docbase.example.org/docs/relative/page",
		},
		{
			name:    "relative document base resolves against argument",
			baseTag: `<base href="/docs/">`,
			baseURL: "https://argument.example.com/blog/",
			want:    "https://argument.example.com/docs/relative/page",
		},
		{
			name:    "argument used when no document base",
			baseTag: "",
			baseURL: "https://argument.example.com/blog/",
			want:    "https://argument.example.com/blog/relative/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article, err := urltomd.ConvertHTML(fmt.Sprintf(relativeLinkHTML, tt.baseTag), tt.baseURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(article.Content, tt.want) {
				t.Errorf("content missing resolved link %q\ngot:\n%s", tt.want, article.Content)
			}
		})
	}
}

func TestConvertHTML_NoArchiveDoesNotTruncateFullArticle(t *testing.T) {
	// Regression: the noarchive signal must be weighed against the extracted
	// article text, not against an empty string, or every page carrying the meta
	// tag is reported truncated regardless of length.
	withNoArchive := strings.Replace(
		articleHTML,
		`<meta charset="UTF-8">`,
		`<meta charset="UTF-8"><meta name="robots" content="noindex, noarchive">`,
		1,
	)

	article, err := urltomd.ConvertHTML(withNoArchive, "https://example.com/blog/post-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if article.IsTruncated {
		t.Errorf("full-length article with noarchive meta reported truncated")
	}

	short := `<html lang="en"><head><meta name="robots" content="noarchive"></head>` +
		`<body><article><h1>Teaser</h1><p>Short preview only.</p></article></body></html>`
	shortArticle, err := urltomd.ConvertHTML(short, "https://example.com/teaser")
	if err != nil {
		t.Fatalf("unexpected error on short article: %v", err)
	}
	if !shortArticle.IsTruncated {
		t.Errorf("short article with noarchive meta not reported truncated")
	}
}
