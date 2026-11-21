package format

import (
	"strings"
	"testing"
)

func TestMarkdownToHTML(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"hello", "hello"},
		{"*italic* and _italic_", "<em>italic</em> and <em>italic</em>"},
		{"**bold**", "<strong>bold</strong>"},
		{"***both***", "<strong><em>both</em></strong>"},
		{"__underline__", "<u>underline</u>"},
		{"~~gone~~", "<del>gone</del>"},
		{"||secret||", "<span data-mx-spoiler>secret</span>"},
		{`\*not italic\*`, "*not italic*"},
		{`a \|\|b\|\|`, "a ||b||"},
		{"snake_case_name stays", "snake_case_name stays"},
		{"2 * 3 * 4", "2 * 3 * 4"},
		{"**bold *and italic* inside**", "<strong>bold <em>and italic</em> inside</strong>"},
		{"*italic **and bold** inside*", "<em>italic <strong>and bold</strong> inside</em>"},
		{"__**underlined bold**__", "<u><strong>underlined bold</strong></u>"},
		{"`code *not* bold`", "<code>code *not* bold</code>"},
		{"use `` a`b `` here", "use <code>a`b</code> here"},
		{"[docs](https://matrix.org)", `<a href="https://matrix.org">docs</a>`},
		{"[x](javascript:alert(1))", "[x](javascript:alert(1))"},
		{"see https://imgur.com/a/abc.", `see <a href="https://imgur.com/a/abc">https://imgur.com/a/abc</a>.`},
		{"(https://example.com/x)", `(<a href="https://example.com/x">https://example.com/x</a>)`},
		{"a <b> & c", "a &lt;b&gt; &amp; c"},
		{"line one\nline two", "line one<br>line two"},
		{"# Title", "<h1>Title</h1>"},
		{"### Small **bold**", "<h3>Small <strong>bold</strong></h3>"},
		{"#hashtag", "#hashtag"},
		{"> quoted\n> more", "<blockquote><p>quoted<br>more</p></blockquote>"},
		{"```go\nfmt.Println(\"<hi>\")\n```", `<pre><code class="language-go">fmt.Println(&#34;&lt;hi&gt;&#34;)</code></pre>`},
		{"- one\n- two", "<ul><li>one</li><li>two</li></ul>"},
		{"1. one\n2. two", "<ol><li>one</li><li>two</li></ol>"},
		{"3. three\n4. four", `<ol start="3"><li>three</li><li>four</li></ol>`},
		{"- a\n  - nested\n- b", "<ul><li>a<ul><li>nested</li></ul></li><li>b</li></ul>"},
		{"intro\n- item", "<p>intro</p><ul><li>item</li></ul>"},
		{"**unclosed", "**unclosed"},
	} {
		if got := HTML(Parse(tc.in)); got != tc.want {
			t.Errorf("HTML(Parse(%q))\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}
}

func TestMentions(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// A bare @name is no address: it goes as the text it is, not as a pill for no one.
		{"@Robin can you check", `@Robin can you check`},
		{"hi @mira:matrix.org.", `hi <a href="https://matrix.to/#/@mira:matrix.org">@mira:matrix.org</a>.`},
		{"mail me at a@b.com", "mail me at a@b.com"},
		{"just @ alone", "just @ alone"},
	} {
		if got := HTML(Parse(tc.in)); got != tc.want && got != "<p>"+tc.want+"</p>" {
			t.Errorf("HTML(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	back := ParseHTML(`<a href="https://matrix.to/#/@jun:kde.org">Jun</a> hello`)
	if m := back.Children[0].Children[0]; m.Kind != Mention || m.Text != "@Jun" {
		t.Fatalf("received mention = %+v", m)
	}
}

// The Markdown we define: the double delimiters format, a lone * or ~ is
// only a character, and a list needs its marker at the line's start.
func TestMarkdownAsDefined(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"~~a~~", "<del>a</del>"},
		{"**bold** and *italic* and __under__", "<strong>bold</strong> and <em>italic</em> and <u>under</u>"},
		{"~a~", "~a~"},
		{"a ~ b ~ c", "a ~ b ~ c"},
		{"*a", "*a"},
		{"a*", "a*"},
		{"2 * 3 * 4", "2 * 3 * 4"},
		{"**a", "**a"},
		{"~~a", "~~a"},
		{"- a", "<ul><li>a</li></ul>"},
		{"- a\n- b", "<ul><li>a</li><li>b</li></ul>"},
		{"-a", "-a"},
		{"a - b", "a - b"},
		{"1. a\n2. b", "<ol><li>a</li><li>b</li></ol>"},
	} {
		if got := HTML(Parse(tc.in)); got != tc.want && got != "<p>"+tc.want+"</p>" {
			t.Errorf("HTML(Parse(%q)) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Plain text is read as typed: no Markdown, but links stay links and the
// lines keep their breaks.
func TestParsePlain(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"- a *b* ~~c~~", "- a *b* ~~c~~"}, // a lone paragraph has no <p>
		{"two\nlines\n\nand a paragraph", "<p>two<br>lines</p><p>and a paragraph</p>"},
		{"see https://example.org/x?y=1 now", `see <a href="https://example.org/x?y=1">https://example.org/x?y=1</a> now`},
		{"x", "x"},
		{"", ""},
	} {
		if got := HTML(ParsePlain(tc.in)); got != tc.want {
			t.Errorf("HTML(ParsePlain(%q)) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// HTML as other clients write it — Element puts a newline between a list's
// items — reads as the same tree: no stray text between the bullets.
func TestParseHTMLAsClientsWriteIt(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<ul>\n<li>a</li>\n<li>b</li>\n</ul>\n", "<ul><li>a</li><li>b</li></ul>"},
		{"<ol start=\"3\">\n<li>x</li>\n</ol>", `<ol start="3"><li>x</li></ol>`},
		{"<p>one <strong>two</strong> <em>three</em> <del>four</del></p>", "one <strong>two</strong> <em>three</em> <del>four</del>"},
		{"first<br />second", "first<br>second"},
	} {
		if got := HTML(ParseHTML(tc.in)); got != tc.want && got != "<p>"+tc.want+"</p>" {
			t.Errorf("HTML(ParseHTML(%q))\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}
}

// A mention chosen from the suggestions is a link to the person with their
// label: one link, not a link around another (which Element shows as two
// pills, one for a user that does not exist).
func TestChosenMentionIsOneLink(t *testing.T) {
	const real = `<a href="https://matrix.to/#/@kf:kaifireborn.com">@KaiFireborn</a>`
	for _, tc := range []struct{ in, want string }{
		{"hi [@KaiFireborn](https://matrix.to/#/@kf:kaifireborn.com) test", "hi " + real + " test"},
		{"😂😂 [@KaiFireborn](https://matrix.to/#/@kf:kaifireborn.com) [@KaiFireborn](https://matrix.to/#/@kf:kaifireborn.com) test",
			"😂😂 " + real + " " + real + " test"},
		// Typed by hand, with the server: a mention. Without it: text.
		{"@kf:kaifireborn.com and @KaiFireborn", `<a href="https://matrix.to/#/@kf:kaifireborn.com">@kf:kaifireborn.com</a> and @KaiFireborn`},
	} {
		got := HTML(Parse(tc.in))
		if got != tc.want && got != "<p>"+tc.want+"</p>" {
			t.Errorf("HTML(%q)\n got %s\nwant %s", tc.in, got, tc.want)
		}
		if strings.Count(got, "<a ") != strings.Count(got, "</a>") || strings.Contains(got, "<a href=\"https://matrix.to/#/@KaiFireborn\"") {
			t.Errorf("nested or bogus anchors in %s", got)
		}
	}
	if Plain(Parse("hi [@KaiFireborn](https://matrix.to/#/@kf:kaifireborn.com)")) != "hi @KaiFireborn" {
		t.Error("the plain text of a mention is its label")
	}
	if NeedsHTML(Parse("just @Robin here")) {
		t.Error("a bare @name needs no formatted body")
	}
}

func TestNeedsHTML(t *testing.T) {
	if NeedsHTML(Parse("just text")) {
		t.Error("plain text needs no formatted_body")
	}
	if !NeedsHTML(Parse("**bold**")) || !NeedsHTML(Parse("a\n\nb")) {
		t.Error("formatting needs a formatted_body")
	}
}

func TestParseHTMLRoundTrip(t *testing.T) {
	for _, md := range []string{
		"**bold** *it* __u__ ~~s~~ ||sp|| `c`",
		"> quote",
		"- a\n- b",
		"1. x\n2. y",
		"```py\nprint(1)\n```",
		"# H",
		"[l](https://x.org)",
	} {
		h := HTML(Parse(md))
		if got := HTML(ParseHTML(h)); got != h && "<p>"+h+"</p>" != got {
			t.Errorf("round trip of %q:\n html %s\n back %s", md, h, got)
		}
	}
}

func TestSanitize(t *testing.T) {
	in := `<p>hi <script>alert(1)</script><a href="javascript:x()">click</a>` +
		`<img src="https://evil/x.png" alt=":wave:"><span style="color:red" data-mx-color="#f00">red</span></p>` +
		`<mx-reply><blockquote>old message</blockquote></mx-reply><iframe src="x"></iframe>`
	got := HTML(ParseHTML(in))
	for _, bad := range []string{"script", "alert", "javascript", "evil", "iframe", "old message", "style"} {
		if strings.Contains(got, bad) {
			t.Errorf("sanitized output has %q: %s", bad, got)
		}
	}
	for _, good := range []string{"hi", "click", ":wave:", "red"} {
		if !strings.Contains(got, good) {
			t.Errorf("sanitized output lost %q: %s", good, got)
		}
	}
}

func TestElementHTML(t *testing.T) {
	// What Element sends for a code block and a spoiler with a reason.
	n := ParseHTML(`<pre><code class="language-rust">fn main() {}
</code></pre><span data-mx-spoiler="plot">twist</span>`)
	if n.Children[0].Kind != CodeBlock || n.Children[0].Lang != "rust" || n.Children[0].Text != "fn main() {}" {
		t.Errorf("code block = %+v", n.Children[0])
	}
	if p := n.Children[1]; p.Kind != Paragraph || p.Children[0].Kind != Spoiler {
		t.Errorf("spoiler = %+v", p)
	}
}

func TestPlain(t *testing.T) {
	got := Plain(Parse("**Hi** ||secret|| `x`\n- one"))
	if got != "Hi (spoiler) x one" {
		t.Errorf("Plain = %q", got)
	}
}
