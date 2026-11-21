package format

import (
	"html"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTML renders n as org.matrix.custom.html for formatted_body. A message
// of one paragraph is sent without <p>, as Element does.
func HTML(n *Node) string {
	var b strings.Builder
	kids := n.Children
	if n.Kind == Doc && len(kids) == 1 && kids[0].Kind == Paragraph {
		kids = kids[0].Children
	}
	for _, ch := range kids {
		writeHTML(&b, ch)
	}
	return b.String()
}

// NeedsHTML reports whether n has formatting worth a formatted_body.
func NeedsHTML(n *Node) bool {
	if n.Kind == Doc && len(n.Children) == 1 && n.Children[0].Kind == Paragraph {
		for _, ch := range n.Children[0].Children {
			if ch.Kind != Text {
				return true
			}
		}
		return false
	}
	return len(n.Children) > 0
}

func writeHTML(b *strings.Builder, n *Node) {
	wrap := func(open, close string) {
		b.WriteString(open)
		for _, ch := range n.Children {
			writeHTML(b, ch)
		}
		b.WriteString(close)
	}
	switch n.Kind {
	case Paragraph:
		wrap("<p>", "</p>")
	case Heading:
		l := strconv.Itoa(min(max(n.Level, 1), 3))
		wrap("<h"+l+">", "</h"+l+">")
	case Quote:
		wrap("<blockquote>", "</blockquote>")
	case CodeBlock:
		b.WriteString("<pre><code")
		if n.Lang != "" {
			b.WriteString(` class="language-` + html.EscapeString(n.Lang) + `"`)
		}
		b.WriteString(">" + html.EscapeString(n.Text) + "</code></pre>")
	case List:
		if n.Ordered {
			start := ""
			if n.Start > 1 {
				start = ` start="` + strconv.Itoa(n.Start) + `"`
			}
			wrap("<ol"+start+">", "</ol>")
		} else {
			wrap("<ul>", "</ul>")
		}
	case Item:
		// A tight item: its single paragraph's inlines, without <p>.
		b.WriteString("<li>")
		for _, ch := range n.Children {
			if ch.Kind == Paragraph {
				for _, in := range ch.Children {
					writeHTML(b, in)
				}
			} else {
				writeHTML(b, ch)
			}
		}
		b.WriteString("</li>")
	case Text:
		b.WriteString(html.EscapeString(n.Text))
	case Bold:
		wrap("<strong>", "</strong>")
	case Italic:
		wrap("<em>", "</em>")
	case Underline:
		wrap("<u>", "</u>")
	case Strike:
		wrap("<del>", "</del>")
	case Spoiler:
		wrap("<span data-mx-spoiler>", "</span>")
	case Code:
		b.WriteString("<code>" + html.EscapeString(n.Text) + "</code>")
	case Link:
		wrap(`<a href="`+html.EscapeString(n.Href)+`">`, "</a>")
	case Break:
		b.WriteString("<br>")
	default:
		wrap("", "")
	}
}

// ParseHTML parses a received formatted_body into the tree, keeping only
// what the timeline shows: unknown tags give their text, scripts, styles,
// images from outside Matrix and reply fallbacks give nothing, and links
// keep only safe targets.
func ParseHTML(s string) *Node {
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(s), ctx)
	doc := &Node{Kind: Doc}
	if err != nil {
		doc.Children = []*Node{{Kind: Paragraph, Children: []*Node{{Kind: Text, Text: s}}}}
		return doc
	}
	var kids []*Node
	for _, n := range nodes {
		kids = append(kids, convert(n)...)
	}
	doc.Children = blockify(kids)
	return doc
}

// blockify wraps runs of inlines among blocks in paragraphs.
func blockify(nodes []*Node) []*Node {
	var out []*Node
	var para *Node
	for _, n := range nodes {
		if n.IsBlock() {
			para = nil
			out = append(out, n)
			continue
		}
		if para == nil {
			if n.Kind == Break || (n.Kind == Text && strings.TrimSpace(n.Text) == "") {
				continue // whitespace between blocks
			}
			para = &Node{Kind: Paragraph}
			out = append(out, para)
		}
		para.Children = append(para.Children, n)
	}
	return out
}

func attr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func convertChildren(n *xhtml.Node) []*Node {
	var out []*Node
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		out = append(out, convert(ch)...)
	}
	return out
}

func convert(n *xhtml.Node) []*Node {
	switch n.Type {
	case xhtml.TextNode:
		if n.Data == "" {
			return nil
		}
		return []*Node{{Kind: Text, Text: n.Data}}
	case xhtml.ElementNode:
	default:
		return nil
	}
	el := func(k Kind) []*Node { return []*Node{{Kind: k, Children: convertChildren(n)}} }
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Head, atom.Title, atom.Iframe, atom.Object, atom.Embed:
		return nil
	case atom.Img:
		// Inline images: only Matrix media, shown by their alt text here.
		if alt, ok := attr(n, "alt"); ok && alt != "" {
			return []*Node{{Kind: Text, Text: alt}}
		}
		return nil
	case atom.P, atom.Div:
		return []*Node{{Kind: Paragraph, Children: inlinesOnly(convertChildren(n))}}
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		l := int(n.Data[1] - '0')
		return []*Node{{Kind: Heading, Level: min(l, 3), Children: inlinesOnly(convertChildren(n))}}
	case atom.Blockquote:
		return []*Node{{Kind: Quote, Children: blockify(convertChildren(n))}}
	case atom.Pre:
		code, lang := textOf(n), ""
		if c := n.FirstChild; c != nil && c.DataAtom == atom.Code {
			if cls, ok := attr(c, "class"); ok {
				for _, f := range strings.Fields(cls) {
					if strings.HasPrefix(f, "language-") {
						lang = strings.ToLower(strings.TrimPrefix(f, "language-"))
					}
				}
			}
		}
		return []*Node{{Kind: CodeBlock, Lang: lang, Text: strings.TrimSuffix(code, "\n")}}
	case atom.Ul, atom.Ol:
		l := &Node{Kind: List, Ordered: n.DataAtom == atom.Ol, Start: 1}
		if s, ok := attr(n, "start"); ok {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				l.Start = v
			}
		}
		for _, ch := range convertChildren(n) {
			if ch.Kind == Item {
				l.Children = append(l.Children, ch)
			}
		}
		return []*Node{l}
	case atom.Li:
		return []*Node{{Kind: Item, Children: blockify(convertChildren(n))}}
	case atom.Strong, atom.B:
		return el(Bold)
	case atom.Em, atom.I:
		return el(Italic)
	case atom.U, atom.Ins:
		return el(Underline)
	case atom.Del, atom.S, atom.Strike:
		return el(Strike)
	case atom.Code:
		return []*Node{{Kind: Code, Text: textOf(n)}}
	case atom.Br:
		return []*Node{{Kind: Break}}
	case atom.A:
		href, _ := attr(n, "href")
		if !SafeURL(href) {
			return convertChildren(n)
		}
		return []*Node{{Kind: Link, Href: href, Children: convertChildren(n)}}
	case atom.Span, atom.Font:
		if _, ok := attr(n, "data-mx-spoiler"); ok {
			return el(Spoiler)
		}
		return convertChildren(n)
	}
	if n.Data == "mx-reply" {
		return nil // the reply's fallback quote: the timeline shows its own
	}
	return convertChildren(n)
}

// inlinesOnly flattens blocks inside an inline context into their text.
func inlinesOnly(nodes []*Node) []*Node {
	var out []*Node
	for _, n := range nodes {
		if n.IsBlock() {
			if len(out) > 0 {
				out = append(out, &Node{Kind: Break})
			}
			out = append(out, inlinesOnly(n.Children)...)
			continue
		}
		out = append(out, n)
	}
	return out
}

func textOf(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		if n.DataAtom == atom.Br {
			b.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
