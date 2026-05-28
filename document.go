package toml

// SyntaxKind identifies the top-level syntactic element represented by a
// SyntaxNode.
type SyntaxKind int

const (
	SyntaxBlankLine SyntaxKind = iota
	SyntaxComment
	SyntaxKeyValue
	SyntaxTable
	SyntaxArrayTable
)

// SyntaxDocument is a parsed TOML document that preserves source layout.
//
// It is intentionally separate from Document, which stores only decoded values.
// SyntaxDocument keeps source spans so future editing APIs can replace only the
// changed TOML syntax while leaving comments, whitespace, and ordering intact.
type SyntaxDocument struct {
	source string
	doc    Document
	nodes  []SyntaxNode
}

// SyntaxNode represents a top-level TOML syntax element.
type SyntaxNode struct {
	Kind       SyntaxKind
	Start      int
	End        int
	KeyStart   int
	KeyEnd     int
	ValueStart int
	ValueEnd   int
	Key        []string
	Raw        string
	RawKey     string
	RawValue   string
}

// ParseDocument parses a TOML document while preserving its source structure.
func ParseDocument(data []byte) (*SyntaxDocument, error) {
	p := newParser(string(data))
	return p.parseSyntaxDocument()
}

// ParseDocumentString parses a TOML document string while preserving its source
// structure.
func ParseDocumentString(s string) (*SyntaxDocument, error) {
	p := newParser(s)
	return p.parseSyntaxDocument()
}

// String returns the original TOML source for an unchanged syntax document.
func (d *SyntaxDocument) String() string {
	if d == nil {
		return ""
	}
	return d.source
}

// Nodes returns the document's top-level syntax nodes.
func (d *SyntaxDocument) Nodes() []SyntaxNode {
	if d == nil {
		return nil
	}
	nodes := make([]SyntaxNode, len(d.nodes))
	copy(nodes, d.nodes)
	for i := range nodes {
		nodes[i].Key = append([]string(nil), nodes[i].Key...)
	}
	return nodes
}

// Document returns the decoded value document associated with the syntax
// document.
func (d *SyntaxDocument) Document() Document {
	if d == nil {
		return nil
	}
	return cloneDocument(d.doc)
}

func cloneDocument(doc Document) Document {
	if doc == nil {
		return nil
	}
	cloned := make(Document, len(doc))
	for key, value := range doc {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(value))
		for key, child := range value {
			cloned[key] = cloneValue(child)
		}
		return cloned
	case []any:
		cloned := make([]any, len(value))
		for i, child := range value {
			cloned[i] = cloneValue(child)
		}
		return cloned
	default:
		return value
	}
}
