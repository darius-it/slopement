// Package format turns messages into a tree the timeline renders, from
// the two sides of a Matrix message:
//
//   - what the user types: Discord-flavoured Markdown (Parse), sent as
//     formatted_body HTML (HTML);
//   - what others sent: org.matrix.custom.html (ParseHTML), sanitized to
//     the same set of nodes.
//
// One tree for both, so a message looks the same whoever wrote it.
//
// The Markdown differs from CommonMark where chat apps do: __x__ is
// underline (not bold), ||x|| is a spoiler, a single newline breaks the
// line, and only # to ### are headings.
package format

// Kind is the kind of a node.
type Kind uint8

const (
	// Blocks.
	Doc Kind = iota
	Paragraph
	Heading // Level 1–3
	Quote
	CodeBlock // Lang, Text
	List      // Ordered, Start
	Item

	// Inlines.
	Text // Text
	Bold
	Italic
	Underline
	Strike
	Spoiler
	Code // Text
	Link // Href
	Break
	// Mention is a user pill: Text is what shows ("@Robin"), Href the
	// matrix.to link of the user ("https://matrix.to/#/@robin:server");
	// typed as "@name" or "@name:server".
	Mention
)

// Node is a node of a message's tree.
type Node struct {
	Kind     Kind
	Text     string // Text, Code, CodeBlock
	Level    int    // Heading
	Lang     string // CodeBlock
	Href     string // Link
	Ordered  bool   // List
	Start    int    // List, when Ordered
	Children []*Node
}

// IsBlock reports whether n is a block.
func (n *Node) IsBlock() bool { return n.Kind <= Item }

// Plain returns n's text without formatting, for notifications, the room
// list's preview and screen readers: spoilers hidden.
func Plain(n *Node) string {
	var b []byte
	var walk func(n *Node, top bool)
	walk = func(n *Node, top bool) {
		switch n.Kind {
		case Text, Code, Mention:
			b = append(b, n.Text...)
		case CodeBlock:
			b = append(b, n.Text...)
		case Break:
			b = append(b, ' ')
		case Spoiler:
			b = append(b, "(spoiler)"...)
			return
		}
		for i, ch := range n.Children {
			if i > 0 && ch.IsBlock() && len(b) > 0 && b[len(b)-1] != ' ' {
				b = append(b, ' ')
			}
			walk(ch, false)
		}
	}
	walk(n, true)
	return string(b)
}
