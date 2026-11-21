package format

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	headingRE = regexp.MustCompile(`^(#{1,3}) +(.*)$`)
	listRE    = regexp.MustCompile(`^( *)([-*]|(\d{1,9})\.) +(.*)$`)
	fenceRE   = regexp.MustCompile("^ {0,3}```\\s*([A-Za-z0-9_+#.-]*)\\s*$")
)

// Parse parses what the user typed.
func Parse(md string) *Node {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	return &Node{Kind: Doc, Children: blocks(strings.Split(md, "\n"))}
}

func blocks(lines []string) []*Node {
	var out []*Node
	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			i++
		case fenceRE.MatchString(line):
			lang := fenceRE.FindStringSubmatch(line)[1]
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) != "```" {
				j++
			}
			out = append(out, &Node{Kind: CodeBlock, Lang: strings.ToLower(lang), Text: strings.Join(lines[i+1:min(j, len(lines))], "\n")})
			i = j + 1
		case strings.HasPrefix(line, "```") && strings.HasSuffix(strings.TrimSpace(line), "```") && len(strings.TrimSpace(line)) > 6:
			// ```one line```
			t := strings.TrimSpace(line)
			out = append(out, &Node{Kind: CodeBlock, Text: t[3 : len(t)-3]})
			i++
		case headingRE.MatchString(line):
			m := headingRE.FindStringSubmatch(line)
			out = append(out, &Node{Kind: Heading, Level: len(m[1]), Children: inline(m[2])})
			i++
		case isQuote(line):
			var inner []string
			for i < len(lines) && isQuote(lines[i]) {
				l := strings.TrimPrefix(lines[i], ">")
				inner = append(inner, strings.TrimPrefix(l, " "))
				i++
			}
			out = append(out, &Node{Kind: Quote, Children: blocks(inner)})
		case listRE.MatchString(line):
			j := i
			for j < len(lines) && (listRE.MatchString(lines[j]) || (j > i && isContinuation(lines[j]))) {
				j++
			}
			out = append(out, lists(lines[i:j])...)
			i = j
		default:
			j := i
			var para []string
			for j < len(lines) && strings.TrimSpace(lines[j]) != "" && (j == i || !startsBlock(lines[j])) {
				para = append(para, lines[j])
				j++
			}
			out = append(out, &Node{Kind: Paragraph, Children: inlineLines(para)})
			i = j
		}
	}
	return out
}

func isQuote(l string) bool { return strings.HasPrefix(l, ">") && !strings.HasPrefix(l, ">>>") }

// isContinuation reports a line that continues a list item: indented text.
func isContinuation(l string) bool {
	return strings.HasPrefix(l, "  ") && strings.TrimSpace(l) != "" && !listRE.MatchString(l)
}

func startsBlock(l string) bool {
	return fenceRE.MatchString(l) || headingRE.MatchString(l) || isQuote(l) || listRE.MatchString(l)
}

// inlineLines joins lines with breaks: a newline in chat is a newline.
func inlineLines(lines []string) []*Node {
	var out []*Node
	for i, l := range lines {
		if i > 0 {
			out = append(out, &Node{Kind: Break})
		}
		out = append(out, inline(l)...)
	}
	return out
}

type listLine struct {
	indent  int
	ordered bool
	num     int
	text    string
}

// lists builds the (nested) lists of consecutive list lines: a line
// indented two or more spaces past its item's starts a list inside it.
func lists(lines []string) []*Node {
	var ls []listLine
	for _, l := range lines {
		m := listRE.FindStringSubmatch(l)
		if m == nil { // continuation of the last item
			if len(ls) > 0 {
				ls[len(ls)-1].text += "\n" + strings.TrimSpace(l)
			}
			continue
		}
		n, _ := strconv.Atoi(m[3])
		ls = append(ls, listLine{indent: len(m[1]), ordered: m[3] != "", num: n, text: m[4]})
	}
	nodes, _ := buildList(ls, 0)
	return nodes
}

func buildList(ls []listLine, i int) ([]*Node, int) {
	var out []*Node
	var cur *Node
	base := ls[i].indent
	for i < len(ls) {
		l := ls[i]
		if l.indent < base {
			break
		}
		if l.indent >= base+2 && cur != nil && len(cur.Children) > 0 {
			item := cur.Children[len(cur.Children)-1]
			sub, next := buildList(ls, i)
			item.Children = append(item.Children, sub...)
			i = next
			continue
		}
		if cur == nil || cur.Ordered != l.ordered {
			cur = &Node{Kind: List, Ordered: l.ordered, Start: max(l.num, 1)}
			out = append(out, cur)
		}
		item := &Node{Kind: Item, Children: []*Node{{Kind: Paragraph, Children: inlineLines(strings.Split(l.text, "\n"))}}}
		cur.Children = append(cur.Children, item)
		i++
	}
	return out, i
}

// delimiters, longest first, and the node each makes.
var delims = []struct {
	s    string
	kind Kind
}{
	{"***", Bold}, // bold + italic
	{"**", Bold},
	{"__", Underline},
	{"~~", Strike},
	{"||", Spoiler},
	{"*", Italic},
	{"_", Italic},
}

func isPunct(b byte) bool {
	return b < utf8.RuneSelf && (unicode.IsPunct(rune(b)) || unicode.IsSymbol(rune(b)))
}

// inline parses a line's inline formatting.
func inline(s string) []*Node {
	var out []*Node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, &Node{Kind: Text, Text: text.String()})
			text.Reset()
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		// Backslash escapes any punctuation.
		if c == '\\' && i+1 < len(s) && isPunct(s[i+1]) {
			text.WriteByte(s[i+1])
			i += 2
			continue
		}
		// Code spans: their content is literal.
		if c == '`' {
			n := run(s, i, '`')
			if end := strings.Index(s[i+n:], s[i:i+n]); end >= 0 && n < 3 {
				flush()
				code := s[i+n : i+n+end]
				if len(code) > 1 && code[0] == ' ' && code[len(code)-1] == ' ' {
					code = code[1 : len(code)-1]
				}
				out = append(out, &Node{Kind: Code, Text: code})
				i += n + end + n
				continue
			}
		}
		// [text](url)
		if c == '[' {
			if label, href, n, ok := link(s[i:]); ok {
				flush()
				// What a link's label says is its text: an "@name" in it is
				// not a mention of its own — "[@Kai](https://matrix.to/#/@kai:x)"
				// is one link, not a link around another.
				out = append(out, &Node{Kind: Link, Href: href, Children: flattenMentions(inline(label))})
				i += n
				continue
			}
		}
		// Bare URLs.
		if (c == 'h' || c == 'H') && (i == 0 || !isWord(s[i-1])) {
			if n := autolink(s[i:]); n > 0 {
				flush()
				u := s[i : i+n]
				out = append(out, &Node{Kind: Link, Href: u, Children: []*Node{{Kind: Text, Text: u}}})
				i += n
				continue
			}
		}
		// @name and @name:server, at a word's start (not an email's middle).
		if c == '@' && (i == 0 || !isWord(s[i-1])) {
			if n := mention(s[i:]); n > 0 {
				flush()
				m := s[i : i+n]
				out = append(out, &Node{Kind: Mention, Text: m, Href: "https://matrix.to/#/" + m})
				i += n
				continue
			}
		}
		if node, n := emphasis(s, i); node != nil {
			flush()
			out = append(out, node)
			i += n
			continue
		}
		text.WriteByte(c)
		i++
	}
	flush()
	return out
}

// flattenMentions turns mention nodes among nodes (and in their children)
// into the text they show.
func flattenMentions(nodes []*Node) []*Node {
	for i, n := range nodes {
		if n.Kind == Mention {
			nodes[i] = &Node{Kind: Text, Text: n.Text}
		} else if len(n.Children) > 0 {
			n.Children = flattenMentions(n.Children)
		}
	}
	return nodes
}

// mention returns the length of an @mention at s's start: @ and a name of
// letters, digits and ._=-, then optionally :server; 0 for none. A
// trailing dot or dash is punctuation, not part of the name.
func mention(s string) int {
	isName := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '=' || r == '-'
	}
	n := 1
	for n < len(s) {
		r, w := utf8.DecodeRuneInString(s[n:])
		if !isName(r) {
			break
		}
		n += w
	}
	if n < len(s) && s[n] == ':' { // a server: host and an optional port
		m := n + 1
		for m < len(s) && (isWord(s[m]) || s[m] == '.' || s[m] == '-' || s[m] == ':') {
			m++
		}
		if m > n+1 && strings.Contains(s[n+1:m], ".") || (m > n+1 && strings.HasPrefix(s[n+1:m], "localhost")) {
			n = m
		}
	}
	for n > 1 && (s[n-1] == '.' || s[n-1] == '-' || s[n-1] == ':') {
		n--
	}
	if n <= 1 {
		return 0
	}
	return n
}

func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

func isWord(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b >= utf8.RuneSelf
}

// emphasis parses a delimited span at s[i:], returning its node and length.
func emphasis(s string, i int) (*Node, int) {
	for _, d := range delims {
		if !strings.HasPrefix(s[i:], d.s) {
			continue
		}
		// _ only opens at a word's start, so snake_case stays text.
		if d.s == "_" && i > 0 && isWord(s[i-1]) {
			return nil, 0
		}
		open := i + len(d.s)
		if open >= len(s) || s[open] == ' ' {
			if len(d.s) > 1 {
				continue // "** " is not an opener; try shorter ones
			}
			return nil, 0
		}
		end := closer(s, open, d.s)
		if end < 0 {
			continue // try a shorter delimiter: "**a*" is * then *a*
		}
		inner := inline(s[open:end])
		n := &Node{Kind: d.kind, Children: inner}
		if d.s == "***" {
			n = &Node{Kind: Bold, Children: []*Node{{Kind: Italic, Children: inner}}}
		}
		return n, end + len(d.s) - i
	}
	return nil, 0
}

// closer finds the delimiter d closing a span opened before s[from:]:
// not after a space, skipping escapes and code, and for a single * or _,
// not part of a longer run (which belongs to a nested span).
func closer(s string, from int, d string) int {
	for j := from; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
			continue
		case '`':
			if end := strings.IndexByte(s[j+1:], '`'); end >= 0 {
				j += end + 1
			}
			continue
		}
		if !strings.HasPrefix(s[j:], d) {
			continue
		}
		if len(d) == 1 {
			// A longer run of the same character belongs to a nested
			// span: skip it whole, wherever it stands.
			r := run(s, j, d[0])
			if r > 1 {
				// A run of 3 closes this one and a nested ** before it.
				if r == 3 && s[j-1] != ' ' {
					return j + 2
				}
				j += r - 1
				continue
			}
			if d == "_" && j+1 < len(s) && isWord(s[j+1]) {
				continue
			}
		}
		if s[j-1] == ' ' {
			continue
		}
		if len(d) == 2 && j+2 < len(s) && s[j+2] == d[0] && d != "||" {
			// "***" closing "**": the * closes a nested span first.
			return j + 1
		}
		if j == from {
			continue
		}
		return j
	}
	return -1
}

// link parses [label](url) at the start of s.
func link(s string) (label, href string, n int, ok bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				if i+1 >= len(s) || s[i+1] != '(' {
					return "", "", 0, false
				}
				end := strings.IndexByte(s[i+2:], ')')
				if end < 0 {
					return "", "", 0, false
				}
				href = strings.TrimSpace(s[i+2 : i+2+end])
				if !SafeURL(href) || strings.ContainsAny(href, " \n") {
					return "", "", 0, false
				}
				return s[1:i], href, i + 2 + end + 1, true
			}
		}
	}
	return "", "", 0, false
}

// autolink returns the length of a bare URL at the start of s, without
// trailing punctuation that ends a sentence.
func autolink(s string) int {
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "https://") && !strings.HasPrefix(low, "http://") {
		return 0
	}
	n := strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '<' || r == '>' })
	if n < 0 {
		n = len(s)
	}
	for n > 0 && strings.ContainsRune(".,:;!?'\"", rune(s[n-1])) {
		n--
	}
	// A closing paren without its opener is the sentence's.
	if n > 0 && s[n-1] == ')' && strings.Count(s[:n], "(") < strings.Count(s[:n], ")") {
		n--
	}
	if n <= len("https://") {
		return 0
	}
	return n
}

// SafeURL reports whether a link's target may be shown: web, mail and
// Matrix links only.
func SafeURL(u string) bool {
	l := strings.ToLower(u)
	for _, p := range []string{"https://", "http://", "mailto:", "matrix:"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}
