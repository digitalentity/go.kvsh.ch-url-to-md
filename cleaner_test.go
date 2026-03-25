package urltomd

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func parseFragment(t *testing.T, src string) *html.Node {
	t.Helper()
	ctx := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Body,
		Data:     "body",
	}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctx)
	if err != nil || len(nodes) == 0 {
		t.Fatalf("parseFragment(%q): %v", src, err)
	}
	return nodes[0]
}

func TestIsNoise_NoiseTags(t *testing.T) {
	for _, tag := range []string{"nav", "aside", "header", "footer", "iframe", "noscript"} {
		t.Run(tag, func(t *testing.T) {
			n := parseFragment(t, "<"+tag+">content</"+tag+">")
			if !isNoise(n) {
				t.Errorf("<%s> should be noise", tag)
			}
		})
	}
}

func TestIsNoise_ArticleTags(t *testing.T) {
	for _, tag := range []string{"article", "section", "p", "h1", "main"} {
		t.Run(tag, func(t *testing.T) {
			n := parseFragment(t, "<"+tag+">content</"+tag+">")
			if isNoise(n) {
				t.Errorf("<%s> should not be noise", tag)
			}
		})
	}
}

func TestIsNoise_NoiseRoles(t *testing.T) {
	for _, role := range []string{"navigation", "banner", "complementary", "search", "contentinfo"} {
		t.Run(role, func(t *testing.T) {
			n := parseFragment(t, `<div role="`+role+`">x</div>`)
			if !isNoise(n) {
				t.Errorf("role=%q should be noise", role)
			}
		})
	}
}

func TestIsNoise_SafeRole(t *testing.T) {
	n := parseFragment(t, `<div role="main">x</div>`)
	if isNoise(n) {
		t.Error(`role="main" should not be noise`)
	}
}

func TestIsNoise_NoiseClass(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"ad", `<div class="ad">x</div>`},
		{"ads", `<div class="ads">x</div>`},
		{"sidebar", `<div class="sidebar">x</div>`},
		{"nav", `<div class="nav">x</div>`},
		{"cookie-banner", `<div class="cookie-banner">x</div>`},
		{"sponsored-content", `<div class="sponsored-content">x</div>`},
		{"related-articles", `<div class="related-articles">x</div>`},
		{"id=newsletter", `<div id="newsletter">x</div>`},
		{"id=footer", `<div id="footer">x</div>`},
		{"social share", `<div class="social share">x</div>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := parseFragment(t, tc.src)
			if !isNoise(n) {
				t.Errorf("%s should be noise", tc.src)
			}
		})
	}
}

func TestIsNoise_NoFalsePositives(t *testing.T) {
	// Words that contain noise substrings but must NOT be flagged.
	cases := []struct {
		name string
		src  string
	}{
		{"loaded", `<div class="loaded">x</div>`},
		{"address", `<div class="address">x</div>`},
		{"gradient", `<div class="gradient">x</div>`},
		{"thread", `<div class="thread">x</div>`},
		{"reading-time", `<div id="reading-time">x</div>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := parseFragment(t, tc.src)
			if isNoise(n) {
				t.Errorf("%s should NOT be noise (false positive)", tc.src)
			}
		})
	}
}

func TestRemoveNoise_DetachesNoisyChildren(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<!DOCTYPE html><html><body>
		<nav>Menu</nav>
		<article><p>Real content here that is long enough.</p></article>
		<footer>Footer text</footer>
	</body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	removeNoise(doc)

	var buf strings.Builder
	if err := html.Render(&buf, doc); err != nil {
		t.Fatal(err)
	}
	rendered := buf.String()

	if strings.Contains(rendered, "Menu") {
		t.Error("nav content should have been removed")
	}
	if strings.Contains(rendered, "Footer text") {
		t.Error("footer content should have been removed")
	}
	if !strings.Contains(rendered, "Real content") {
		t.Error("article content should be preserved")
	}
}
