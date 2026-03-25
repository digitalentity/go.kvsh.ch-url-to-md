package urltomd

import (
	"strings"

	"golang.org/x/net/html"
)

// noiseTags are semantic elements whose entire subtree is considered noise.
var noiseTags = map[string]bool{
	"nav":      true,
	"aside":    true,
	"header":   true,
	"footer":   true,
	"iframe":   true,
	"noscript": true,
}

// noiseRoles are ARIA landmark roles that indicate non-article content.
var noiseRoles = map[string]bool{
	"navigation":    true,
	"banner":        true,
	"complementary": true,
	"search":        true,
	"contentinfo":   true,
}

// noisePatterns are matched against individual tokens from class and id values.
// Tokens are split on spaces, hyphens, and underscores to avoid false positives
// (e.g. "ad" must not match "address" or "load").
var noisePatterns = map[string]bool{
	"ad": true, "ads": true, "advert": true, "advertisement": true, "sponsor": true, "sponsored": true,
	"banner": true, "promo": true,
	"nav": true, "navigation": true, "menu": true, "breadcrumb": true,
	"sidebar": true, "widget": true,
	"cookie": true, "consent": true, "gdpr": true,
	"popup": true, "modal": true, "overlay": true,
	"related": true, "recommended": true, "trending": true, "popular": true,
	"social": true, "share": true, "follow": true, "subscribe": true, "newsletter": true,
	"partner": true, "affiliate": true,
	"comment": true, "disqus": true,
	"footer": true, "header": true,
}

// cleanDOM removes known noise nodes from the parsed HTML tree in-place.
func cleanDOM(doc *html.Node) {
	removeNoise(doc)
}

// removeNoise walks the tree and detaches noise nodes.
func removeNoise(n *html.Node) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		if isNoise(c) {
			n.RemoveChild(c)
			continue
		}
		removeNoise(c)
	}
}

func isNoise(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if noiseTags[n.Data] {
		return true
	}
	for _, a := range n.Attr {
		switch a.Key {
		case "role":
			if noiseRoles[strings.ToLower(a.Val)] {
				return true
			}
		case "class", "id":
			if matchesNoisePattern(a.Val) {
				return true
			}
		}
	}
	return false
}

// matchesNoisePattern reports whether any token in val exactly matches a noise
// pattern. Tokens are split on spaces, hyphens, and underscores.
func matchesNoisePattern(val string) bool {
	lower := strings.ToLower(val)
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})
	for _, tok := range tokens {
		if noisePatterns[tok] {
			return true
		}
	}
	return false
}
