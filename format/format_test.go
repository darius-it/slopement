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
		{"@Robin can you check", `<a href="https://matrix.to/#/@Robin">@Robin</a> can you check`},
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
