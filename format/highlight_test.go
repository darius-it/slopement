package format

import (
	"strings"
	"testing"
)

func TestHighlight(t *testing.T) {
	for _, tc := range []struct {
		name, lang, code string
		want             []CodeToken
	}{
		{"go", "go", "x := 42 // answer\nreturn \"ns\"",
			[]CodeToken{{CodePlain, "x := "}, {CodeNumber, "42"}, {CodePlain, " "}, {CodeComment, "// answer"}, {CodePlain, "\n"}, {CodeKeyword, "return"}, {CodePlain, " "}, {CodeString, "\"ns\""}}},
		{"go raw string", "golang", "s := `a // b`",
			[]CodeToken{{CodePlain, "s := "}, {CodeString, "`a // b`"}}},
		{"comment opener in a string", "go", `"http://x"`,
			[]CodeToken{{CodeString, "\"http://x\""}}},
		{"python docstring", "python", "def f():\n    \"\"\"docs\n    more\"\"\"\n    return 1",
			[]CodeToken{{CodeKeyword, "def"}, {CodePlain, " f():\n    "}, {CodeString, "\"\"\"docs\n    more\"\"\""}, {CodePlain, "\n    "}, {CodeKeyword, "return"}, {CodePlain, " "}, {CodeNumber, "1"}}},
		{"sql folds case", "sql", "SELECT * FROM t -- all",
			[]CodeToken{{CodeKeyword, "SELECT"}, {CodePlain, " * "}, {CodeKeyword, "FROM"}, {CodePlain, " t "}, {CodeComment, "-- all"}}},
		{"unknown language", "bbcode", "[b]hi[/b] 42",
			[]CodeToken{{CodePlain, "[b]hi[/b] 42"}}},
		{"unterminated block comment", "c", "int x; /* note",
			[]CodeToken{{CodeKeyword, "int"}, {CodePlain, " x; "}, {CodeComment, "/* note"}}},
		{"empty", "go", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Highlight(tc.lang, tc.code)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v\nwant %v", got, tc.want)
			}
			var joined strings.Builder
			for i, tok := range got {
				if tok != tc.want[i] {
					t.Errorf("token %d = %v, want %v", i, tok, tc.want[i])
				}
				joined.WriteString(tok.Text)
			}
			if joined.String() != tc.code {
				t.Errorf("tokens changed the code: %q", joined.String())
			}
		})
	}
}

// Fuzz-like check: highlighting never drops, adds or reorders a byte, and
// never makes an empty run.
func TestHighlightPreservesCode(t *testing.T) {
	samples := []string{
		"", "\n", `"`, `'`, "/*", "`", "a\"b\\c\"d",
		"package main\n\nfunc main() {\n\tprintln(\"hi\", 0x1F, 3.14)\n}\n",
		"# comment\nvalue: 'it\\'s'\n",
		"```\nnested\n```",
	}
	langs := []string{"", "go", "python", "js", "yaml", "sql", "unknown"}
	for _, lang := range langs {
		for _, code := range samples {
			var joined strings.Builder
			for _, tok := range Highlight(lang, code) {
				if tok.Text == "" {
					t.Errorf("%s %q: empty run", lang, code)
				}
				joined.WriteString(tok.Text)
			}
			if joined.String() != code {
				t.Errorf("%s %q: got %q", lang, code, joined.String())
			}
		}
	}
}
