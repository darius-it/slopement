package format

import "strings"

// CodeKind is what a run of code is, for coloring it.
type CodeKind uint8

const (
	CodePlain CodeKind = iota
	CodeKeyword
	CodeString
	CodeComment
	CodeNumber
)

// CodeToken is a run of code of one kind. The runs' texts, concatenated,
// are the code unchanged.
type CodeToken struct {
	Kind CodeKind
	Text string
}

// codeLang is a language's lexical rules: how it comments and quotes, and
// its keywords. It is a tokenizer, not a parser, so it stays small and can
// never fail: what it doesn't know is plain.
type codeLang struct {
	line   []string  // line comment openers, longest first
	block  [2]string // block comment open and close, optional
	raw    []string  // multiline string delimiters (Go's `, Python's """)
	single bool      // '…' is a string too (not just "…")
	fold   bool      // keywords match case-insensitively (SQL)
	words  string    // keywords, space-separated
}

var codeLangs = map[string]codeLang{
	"go": {line: []string{"//"}, block: [2]string{"/*", "*/"}, raw: []string{"`"}, single: true,
		words: "break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var nil true false iota"},
	"rust": {line: []string{"//"}, block: [2]string{"/*", "*/"},
		words: "as async await break const continue crate dyn else enum extern fn for if impl in let loop match mod move mut pub ref return self static struct super trait type unsafe use where while true false Self"},
	"js": {line: []string{"//"}, block: [2]string{"/*", "*/"}, raw: []string{"`"}, single: true,
		words: "async await break case catch class const continue debugger default delete do else export extends finally for from function if import in instanceof let new of return static super switch this throw try typeof var void while with yield true false null undefined"},
	"ts": {line: []string{"//"}, block: [2]string{"/*", "*/"}, raw: []string{"`"}, single: true,
		words: "abstract any as async await boolean break case catch class const continue declare default delete do else enum export extends finally for from function if implements import in infer interface is keyof let namespace never new number object of private protected public readonly return static string super switch this throw try type typeof undefined unknown var void while with yield true false null"},
	"python": {line: []string{"#"}, raw: []string{`"""`, `'''`}, single: true,
		words: "and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield True False None self"},
	"bash": {line: []string{"#"}, single: true,
		words: "if then else elif fi for while until do done case esac function in select return local export readonly shift unset source alias"},
	"json": {single: false, words: "true false null"},
	"yaml": {line: []string{"#"}, single: true, words: "true false null"},
	"c": {line: []string{"//"}, block: [2]string{"/*", "*/"}, single: true,
		words: "auto break case char const continue default do double else enum extern float for goto if inline int long register return short signed sizeof static struct switch typedef union unsigned void volatile while bool true false NULL"},
	"cpp": {line: []string{"//"}, block: [2]string{"/*", "*/"}, single: true,
		words: "auto bool break case catch char class const constexpr continue default delete do double else enum explicit extern false float for friend goto if inline int long namespace new nullptr operator private protected public return short signed sizeof static struct switch template this throw true try typedef typename union unsigned using virtual void volatile while"},
	"java": {line: []string{"//"}, block: [2]string{"/*", "*/"}, single: true,
		words: "abstract assert boolean break byte case catch char class const continue default do double else enum extends final finally float for if implements import instanceof int interface long native new package private protected public return short static strictfp super switch synchronized this throw throws transient try var void volatile while true false null"},
	"sql": {line: []string{"--"}, block: [2]string{"/*", "*/"}, single: true, fold: true,
		words: "select from where insert into values update set delete create table drop alter join left right inner outer on as and or not null primary key foreign references group by order having limit offset distinct union all exists between like in is default index view true false"},
}

// codeAliases maps the labels people put on fences to a language.
var codeAliases = map[string]string{
	"golang": "go", "rs": "rust",
	"javascript": "js", "jsx": "js", "node": "js", "mjs": "js",
	"typescript": "ts", "tsx": "ts",
	"py": "python", "python3": "python",
	"sh": "bash", "shell": "bash", "zsh": "bash", "console": "bash",
	"yml": "yaml", "c++": "cpp", "cc": "cpp", "cxx": "cpp", "h": "c", "hpp": "cpp",
}

// Highlight splits code in lang (the fence's label, as "go" or "python")
// into runs to color. An unknown language comes back as one plain run.
// It colors only what is unambiguous — comments, strings, numbers and
// keywords — so the result stays quiet rather than rainbow.
func Highlight(lang, code string) []CodeToken {
	if code == "" {
		return nil
	}
	l, ok := codeLangs[strings.ToLower(lang)]
	if !ok {
		l, ok = codeLangs[codeAliases[strings.ToLower(lang)]]
	}
	if !ok {
		return []CodeToken{{Text: code}}
	}
	var (
		out  []CodeToken
		from int // start of the plain run being accumulated
	)
	emit := func(kind CodeKind, text string) {
		if text == "" {
			return
		}
		if n := len(out); n > 0 && out[n-1].Kind == kind {
			out[n-1].Text += text
			return
		}
		out = append(out, CodeToken{kind, text})
	}
	for i := 0; i < len(code); {
		rest := code[i:]
		// A line comment ends the line; the newline itself stays plain.
		if _, ok := hasAny(rest, l.line); ok {
			emit(CodePlain, code[from:i])
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				end = len(rest)
			}
			emit(CodeComment, rest[:end])
			i += end
			from = i
			continue
		}
		if l.block[0] != "" && strings.HasPrefix(rest, l.block[0]) {
			emit(CodePlain, code[from:i])
			end := strings.Index(rest[len(l.block[0]):], l.block[1])
			if end < 0 {
				emit(CodeComment, rest)
				i = len(code)
			} else {
				end += len(l.block[0]) + len(l.block[1])
				emit(CodeComment, rest[:end])
				i += end
			}
			from = i
			continue
		}
		if raw, ok := hasAny(rest, l.raw); ok {
			emit(CodePlain, code[from:i])
			end := strings.Index(rest[len(raw):], raw)
			if end < 0 {
				emit(CodeString, rest)
				i = len(code)
			} else {
				end += 2 * len(raw)
				emit(CodeString, rest[:end])
				i += end
			}
			from = i
			continue
		}
		ch := rest[0]
		if ch == '"' || ch == '\'' && l.single {
			emit(CodePlain, code[from:i])
			end := quoted(rest, ch)
			emit(CodeString, rest[:end])
			i += end
			from = i
			continue
		}
		if isDigit(ch) {
			emit(CodePlain, code[from:i])
			j := i + 1
			for j < len(code) && (isAlnum(code[j]) || code[j] == '.' || code[j] == '_') {
				j++
			}
			emit(CodeNumber, code[i:j])
			i, from = j, j
			continue
		}
		if isLetter(ch) || ch == '_' {
			j := i + 1
			for j < len(code) && (isAlnum(code[j]) || code[j] == '_') {
				j++
			}
			if isKeyword(l, code[i:j]) {
				emit(CodePlain, code[from:i])
				emit(CodeKeyword, code[i:j])
				from = j
			}
			i = j
			continue
		}
		i++
	}
	emit(CodePlain, code[from:])
	return out
}

// hasAny reports which of the delimiters opens rest, if one does.
func hasAny(s string, delims []string) (string, bool) {
	for _, d := range delims {
		if strings.HasPrefix(s, d) {
			return d, true
		}
	}
	return "", false
}

// quoted is the length of the quoted string opening s, escapes respected;
// an unterminated one runs to the end of its line.
func quoted(s string, q byte) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case q:
			return i + 1
		case '\n':
			return i
		}
	}
	return len(s)
}

func isKeyword(l codeLang, w string) bool {
	if l.fold {
		w = strings.ToLower(w)
	}
	for rest := l.words; rest != ""; {
		var kw string
		kw, rest, _ = strings.Cut(rest, " ")
		if kw == w {
			return true
		}
	}
	return false
}

func isLetter(ch byte) bool { return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' }
func isDigit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func isAlnum(ch byte) bool  { return isLetter(ch) || isDigit(ch) }
