package catalog

import (
	nethtml "golang.org/x/net/html"
	"html"
	"strings"
)

// No attributes, URLs, styles or embedded media are allowed in product prose.
func Sanitize(source string) (string, error) {
	doc, e := nethtml.Parse(strings.NewReader(source))
	if e != nil {
		return "", e
	}
	allow := map[string]bool{"p": true, "br": true, "strong": true, "b": true, "em": true, "i": true, "ul": true, "ol": true, "li": true, "h2": true, "h3": true, "blockquote": true}
	blocked := map[string]bool{"script": true, "style": true, "iframe": true, "svg": true, "math": true, "object": true, "embed": true, "template": true}
	var out strings.Builder
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && blocked[n.Data] {
			return
		}
		if n.Type == nethtml.TextNode {
			out.WriteString(html.EscapeString(n.Data))
		}
		tag := n.Type == nethtml.ElementNode && allow[n.Data]
		if tag {
			out.WriteString("<" + n.Data + ">")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if tag && n.Data != "br" {
			out.WriteString("</" + n.Data + ">")
		}
	}
	walk(doc)
	return out.String(), nil
}
