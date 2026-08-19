package urltomd

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

var paywallPhrases = map[string][]string{
	"en": {
		"subscriber-only",
		"to continue reading",
		"subscribe now",
		"sign in to read",
	},
}

// detectChallengeResponse checks HTTP headers, status codes, and HTML content for challenge signatures.
func detectChallengeResponse(resp *http.Response, doc *html.Node) error {
	if resp == nil {
		if doc != nil {
			return detectChallengeDoc(doc)
		}
		return nil
	}

	// 1. Check Cloudflare mitigation header
	if strings.EqualFold(resp.Header.Get("cf-mitigated"), "challenge") {
		return ErrChallengeBlocked
	}

	// 2. Check challenge markers in HTML document
	if doc != nil {
		if err := detectChallengeDoc(doc); err != nil {
			return err
		}
	}

	// 3. Check status code combined with CDN signatures
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if isCDNSignature(resp.Header) {
			return ErrChallengeBlocked
		}
	}

	return nil
}

// detectChallengeDoc inspects HTML document title and body for challenge/interstitial markers.
func detectChallengeDoc(doc *html.Node) error {
	if doc == nil {
		return nil
	}
	title := strings.ToLower(extractTitle(doc))
	if strings.Contains(title, "just a moment...") ||
		strings.Contains(title, "attention required! | cloudflare") ||
		strings.Contains(title, "security check") ||
		strings.Contains(title, "human verification") ||
		strings.Contains(title, "cloudflare turnstile") ||
		strings.Contains(title, "ddos protection by cloudflare") {
		return ErrChallengeBlocked
	}
	return nil
}

func isCDNSignature(h http.Header) bool {
	if h.Get("cf-ray") != "" || strings.Contains(strings.ToLower(h.Get("server")), "cloudflare") {
		return true
	}
	if h.Get("x-akamai-transformed") != "" || strings.Contains(strings.ToLower(h.Get("server")), "akamaighost") {
		return true
	}
	if h.Get("x-iinfo") != "" || strings.Contains(strings.ToLower(h.Get("x-cdn")), "imperva") {
		return true
	}
	return false
}

// paywallSignals holds document-level truncation signals captured before noise
// removal, which strips the <script> and <meta> nodes they are read from.
type paywallSignals struct {
	jsonLDPaywalled bool
	noArchive       bool
	docLang         string
}

// collectPaywallSignals reads the document-level signals that must be sampled
// before cleanDOM runs. It does not decide truncation on its own: the short-body
// and lexical tiers need the extracted article text, which is not available yet.
func collectPaywallSignals(doc *html.Node) paywallSignals {
	if doc == nil {
		return paywallSignals{}
	}
	isPaywalled, found := checkJSONLDPaywall(doc)
	return paywallSignals{
		jsonLDPaywalled: found && isPaywalled,
		noArchive:       hasNoArchiveMeta(doc),
		docLang:         extractDocLanguage(doc),
	}
}

// truncated evaluates the collected signals against the extracted article text.
func (s paywallSignals) truncated(articleText string, detectedLang string) bool {
	// Tier A: Structural signals (language-agnostic).
	// 1. JSON-LD isAccessibleForFree.
	if s.jsonLDPaywalled {
		return true
	}

	// 2. robots noarchive combined with short body (< 300 chars).
	if s.noArchive && len(strings.TrimSpace(articleText)) < 300 {
		return true
	}

	// Tier B: Lexical signals (language-gated).
	lang := primaryLanguage(detectedLang)
	if lang == "" {
		lang = primaryLanguage(s.docLang)
	}
	if lang != "" && lang != "en" {
		return false
	}
	lowerText := strings.ToLower(articleText)
	for _, phrase := range paywallPhrases["en"] {
		if strings.Contains(lowerText, phrase) {
			return true
		}
	}

	return false
}

// detectTruncated determines if the article is truncated or paywalled. It samples
// and evaluates in one step, so callers that clean the document must instead use
// collectPaywallSignals before cleaning and truncated after extraction.
func detectTruncated(doc *html.Node, articleText string, detectedLang string) bool {
	return collectPaywallSignals(doc).truncated(articleText, detectedLang)
}

func primaryLanguage(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if i := strings.IndexAny(lang, "-_"); i != -1 {
		return lang[:i]
	}
	return lang
}

func extractTitle(doc *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			title = extractText(n)
			return
		}
		for c := n.FirstChild; c != nil && title == ""; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(title)
}

func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(curr *html.Node) {
		if curr.Type == html.TextNode {
			sb.WriteString(curr.Data)
		}
		for c := curr.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func extractDocLanguage(doc *html.Node) string {
	var lang string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "html" {
				for _, a := range n.Attr {
					if strings.EqualFold(a.Key, "lang") || strings.EqualFold(a.Key, "xml:lang") {
						lang = a.Val
						return
					}
				}
			}
			if n.Data == "meta" {
				var isLangMeta, isOGLocale bool
				var content string
				for _, a := range n.Attr {
					if strings.EqualFold(a.Key, "http-equiv") && strings.EqualFold(a.Val, "content-language") {
						isLangMeta = true
					}
					if strings.EqualFold(a.Key, "property") && strings.EqualFold(a.Val, "og:locale") {
						isOGLocale = true
					}
					if strings.EqualFold(a.Key, "content") {
						content = a.Val
					}
				}
				if (isLangMeta || isOGLocale) && content != "" && lang == "" {
					lang = content
				}
			}
		}
		for c := n.FirstChild; c != nil && lang == ""; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(lang)
}

func hasNoArchiveMeta(doc *html.Node) bool {
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var isRobots bool
			var hasNoArchive bool
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "name") && strings.EqualFold(a.Val, "robots") {
					isRobots = true
				}
				if strings.EqualFold(a.Key, "content") && strings.Contains(strings.ToLower(a.Val), "noarchive") {
					hasNoArchive = true
				}
			}
			if isRobots && hasNoArchive {
				found = true
				return
			}
		}
		for c := n.FirstChild; c != nil && !found; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

func checkJSONLDPaywall(doc *html.Node) (isPaywalled bool, found bool) {
	var scripts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "type") && strings.EqualFold(strings.TrimSpace(a.Val), "application/ld+json") {
					scripts = append(scripts, extractText(n))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	for _, scriptContent := range scripts {
		scriptContent = strings.TrimSpace(scriptContent)
		if scriptContent == "" {
			continue
		}
		var parsed any
		if err := json.Unmarshal([]byte(scriptContent), &parsed); err != nil {
			continue
		}
		if paywalled, ok := searchIsAccessibleForFree(parsed); ok {
			return paywalled, true
		}
	}
	return false, false
}

func searchIsAccessibleForFree(v any) (isPaywalled bool, found bool) {
	switch val := v.(type) {
	case map[string]any:
		for k, item := range val {
			if strings.EqualFold(k, "isAccessibleForFree") {
				switch b := item.(type) {
				case bool:
					return !b, true
				case string:
					s := strings.ToLower(strings.TrimSpace(b))
					if s == "false" || s == "0" {
						return true, true
					}
					if s == "true" || s == "1" {
						return false, true
					}
				case float64:
					if b == 0 {
						return true, true
					}
					if b == 1 {
						return false, true
					}
				}
			}
		}
		for _, item := range val {
			if paywalled, ok := searchIsAccessibleForFree(item); ok {
				return paywalled, true
			}
		}
	case []any:
		for _, item := range val {
			if paywalled, ok := searchIsAccessibleForFree(item); ok {
				return paywalled, true
			}
		}
	}
	return false, false
}
