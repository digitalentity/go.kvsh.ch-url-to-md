package urltomd_test

import (
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

func TestConvert_InvalidURL(t *testing.T) {
	_, err := urltomd.Convert("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestConvert_NonHTMLContentType(t *testing.T) {
	srv := serve(t, `{"key":"value"}`, "application/json")
	defer srv.Close()

	_, err := urltomd.Convert(srv.URL)
	if err == nil {
		t.Fatal("expected error for non-HTML content type, got nil")
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

func TestConvert_NoExcessiveBlankLines(t *testing.T) {
	srv := serve(t, articleHTML, "text/html; charset=utf-8")
	defer srv.Close()

	article, err := urltomd.Convert(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	consecutive := 0
	for _, l := range strings.Split(article.Content, "\n") {
		if strings.TrimSpace(l) == "" {
			consecutive++
			if consecutive > 1 {
				t.Error("found more than one consecutive blank line in output")
				break
			}
		} else {
			consecutive = 0
		}
	}
}
