package urltomd

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parseHTMLString(t *testing.T, raw string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}
	return doc
}

func TestDetectChallengeDoc(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantErr error
	}{
		{
			name:    "cloudflare just a moment title",
			html:    `<html><head><title>Just a moment...</title></head><body>Checking your browser</body></html>`,
			wantErr: ErrChallengeBlocked,
		},
		{
			name:    "cloudflare attention required title",
			html:    `<html><head><title>Attention Required! | Cloudflare</title></head><body>Captcha</body></html>`,
			wantErr: ErrChallengeBlocked,
		},
		{
			name:    "security check title",
			html:    `<html><head><title>Security Check</title></head><body>Verify you are human</body></html>`,
			wantErr: ErrChallengeBlocked,
		},
		{
			name:    "normal article title",
			html:    `<html><head><title>An Interesting Article</title></head><body>Content here</body></html>`,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseHTMLString(t, tt.html)
			err := detectChallengeDoc(doc)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("detectChallengeDoc() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectChallengeResponse(t *testing.T) {
	tests := []struct {
		name    string
		resp    *http.Response
		html    string
		wantErr error
	}{
		{
			name: "cf-mitigated challenge header",
			resp: &http.Response{
				StatusCode: 403,
				Header: http.Header{
					"Cf-Mitigated": []string{"challenge"},
				},
			},
			html:    `<html><body>Challenge</body></html>`,
			wantErr: ErrChallengeBlocked,
		},
		{
			name: "403 with cloudflare server header",
			resp: &http.Response{
				StatusCode: 403,
				Header: http.Header{
					"Server": []string{"cloudflare"},
					"Cf-Ray": []string{"123456"},
				},
			},
			html:    `<html><body>Access Denied</body></html>`,
			wantErr: ErrChallengeBlocked,
		},
		{
			name: "429 with akamai ghost",
			resp: &http.Response{
				StatusCode: 429,
				Header: http.Header{
					"Server": []string{"AkamaiGHost"},
				},
			},
			html:    `<html><body>Too Many Requests</body></html>`,
			wantErr: ErrChallengeBlocked,
		},
		{
			name: "403 without CDN signature",
			resp: &http.Response{
				StatusCode: 403,
				Header:     http.Header{},
			},
			html:    `<html><body>Forbidden</body></html>`,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseHTMLString(t, tt.html)
			err := detectChallengeResponse(tt.resp, doc)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("detectChallengeResponse() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectTruncated_JSONLD(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		wantTruncated bool
	}{
		{
			name: "jsonld isAccessibleForFree boolean false",
			html: `<html><head>
			<script type="application/ld+json">
			{
				"@context": "https://schema.org",
				"@type": "NewsArticle",
				"headline": "Exclusive Story",
				"isAccessibleForFree": false
			}
			</script>
			</head><body><p>Short preview of story</p></body></html>`,
			wantTruncated: true,
		},
		{
			name: "jsonld isAccessibleForFree string False in @graph",
			html: `<html><head>
			<script type="application/ld+json">
			{
				"@context": "https://schema.org",
				"@graph": [
					{
						"@type": "WebPage",
						"name": "Page"
					},
					{
						"@type": "NewsArticle",
						"isAccessibleForFree": "False"
					}
				]
			}
			</script>
			</head><body><p>Short preview of story</p></body></html>`,
			wantTruncated: true,
		},
		{
			name: "jsonld isAccessibleForFree true",
			html: `<html><head>
			<script type="application/ld+json">
			{
				"@context": "https://schema.org",
				"@type": "NewsArticle",
				"headline": "Free Story",
				"isAccessibleForFree": true
			}
			</script>
			</head><body><p>This is a full free article with lots of text that is not truncated.</p></body></html>`,
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseHTMLString(t, tt.html)
			truncated := detectTruncated(doc, "article content", "en")
			if truncated != tt.wantTruncated {
				t.Errorf("detectTruncated() = %v, want %v", truncated, tt.wantTruncated)
			}
		})
	}
}

func TestDetectTruncated_RobotsNoArchive(t *testing.T) {
	shortBody := "Very short text preview under 300 characters."
	longBody := strings.Repeat("Long article body text repeating words to easily exceed three hundred characters limit for testing purposes. ", 5)

	noArchiveHTML := `<html><head><meta name="robots" content="noindex, noarchive"></head><body>Content</body></html>`
	doc := parseHTMLString(t, noArchiveHTML)

	if !detectTruncated(doc, shortBody, "en") {
		t.Errorf("expected noarchive + short body to be truncated")
	}

	if detectTruncated(doc, longBody, "en") {
		t.Errorf("expected noarchive + long body NOT to be truncated")
	}
}

func TestDetectTruncated_LanguageGating(t *testing.T) {
	enDoc := parseHTMLString(t, `<html lang="en"><body>Article</body></html>`)
	deDoc := parseHTMLString(t, `<html lang="de"><body>Artikel</body></html>`)
	noLangDoc := parseHTMLString(t, `<html><body>Article</body></html>`)

	paywallText := "Please subscribe now to continue reading the remainder of this article."

	if !detectTruncated(enDoc, paywallText, "en") {
		t.Errorf("expected English text with subscribe phrase to be truncated")
	}

	if !detectTruncated(noLangDoc, paywallText, "") {
		t.Errorf("expected absent lang with subscribe phrase to fall back to English detection")
	}

	if detectTruncated(deDoc, paywallText, "de") {
		t.Errorf("expected non-English (de) document to skip English phrase matching")
	}
}
