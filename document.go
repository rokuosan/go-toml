package toml

import (
	"fmt"
	"reflect"
	"strings"
)

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
	Path       []string
	Value      any
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
		nodes[i].Path = append([]string(nil), nodes[i].Path...)
		nodes[i].Value = cloneValue(nodes[i].Value)
	}
	return nodes
}

// Set replaces the existing scalar value at a dotted path while preserving the
// surrounding TOML source.
func (d *SyntaxDocument) Set(path string, value any) error {
	if path == "" {
		return fmt.Errorf("toml: path must not be empty")
	}
	return d.SetPath(strings.Split(path, "."), value)
}

// SetPath replaces the existing scalar value at path while preserving the
// surrounding TOML source.
func (d *SyntaxDocument) SetPath(path []string, value any) error {
	if d == nil {
		return fmt.Errorf("toml: cannot set value on nil document")
	}
	if len(path) == 0 {
		return fmt.Errorf("toml: path must not be empty")
	}
	encoded, err := encodeDocumentScalarValue(reflect.ValueOf(value))
	if err != nil {
		return err
	}
	decoded, err := parseDocumentScalarValue(encoded)
	if err != nil {
		return err
	}
	matches := d.findScalarNodes(path)
	if len(matches) == 0 {
		return fmt.Errorf("toml: path %s not found", strings.Join(path, "."))
	}
	if len(matches) > 1 {
		return fmt.Errorf("toml: path %s is ambiguous", strings.Join(path, "."))
	}
	i := matches[0]
	node := d.nodes[i]
	if err := setDocumentValue(d.doc, node.Path, decoded); err != nil {
		return err
	}
	d.replaceValue(i, encoded)
	return nil
}

func (d *SyntaxDocument) findScalarNodes(path []string) []int {
	var matches []int
	for i := range d.nodes {
		node := &d.nodes[i]
		if node.Kind != SyntaxKeyValue || !samePath(node.Path, path) || !isScalarDocumentValue(node.Value) {
			continue
		}
		matches = append(matches, i)
	}
	return matches
}

func (d *SyntaxDocument) replaceValue(nodeIndex int, value string) {
	node := d.nodes[nodeIndex]
	oldLen := node.ValueEnd - node.ValueStart
	d.source = d.source[:node.ValueStart] + value + d.source[node.ValueEnd:]
	delta := len(value) - oldLen
	node.ValueEnd = node.ValueStart + len(value)
	node.End += delta
	node.Raw = d.source[node.Start:node.End]
	node.RawValue = value
	node.Value = d.valueAtPath(node.Path)
	d.nodes[nodeIndex] = node
	for i := nodeIndex + 1; i < len(d.nodes); i++ {
		shiftNode(&d.nodes[i], delta)
	}
}

func shiftNode(node *SyntaxNode, delta int) {
	node.Start += delta
	node.End += delta
	if node.KeyStart != 0 || node.KeyEnd != 0 {
		node.KeyStart += delta
		node.KeyEnd += delta
	}
	if node.ValueStart != 0 || node.ValueEnd != 0 {
		node.ValueStart += delta
		node.ValueEnd += delta
	}
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

func encodeDocumentScalarValue(v reflect.Value) (string, error) {
	v = indirectValue(v)
	if !v.IsValid() {
		return "", fmt.Errorf("toml: nil has no TOML representation")
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return "", fmt.Errorf("toml: value type %s is not a scalar", v.Type())
	case reflect.Struct:
		if !isStructScalarType(v) {
			if isTextMarshalerValue(v) {
				break
			}
			return "", fmt.Errorf("toml: value type %s is not a scalar", v.Type())
		}
	}
	return encodeScalarValue(v)
}

func parseDocumentScalarValue(s string) (any, error) {
	p := newParser(s)
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	if !p.eof() {
		return nil, p.err("expected end of value")
	}
	return value, nil
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isScalarDocumentValue(v any) bool {
	switch v.(type) {
	case nil, map[string]any, []any:
		return false
	default:
		return true
	}
}

func (d *SyntaxDocument) valueAtPath(path []string) any {
	var current any = map[string]any(d.doc)
	for _, part := range path {
		m, ok := singleDocumentMap(current)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

func setDocumentValue(doc Document, path []string, value any) error {
	m, ok := singleDocumentMap(map[string]any(doc))
	if !ok {
		return fmt.Errorf("toml: path %s not found", strings.Join(path, "."))
	}
	for _, part := range path[:len(path)-1] {
		next, ok := singleDocumentMap(m[part])
		if !ok {
			return fmt.Errorf("toml: path %s not found", strings.Join(path, "."))
		}
		m = next
	}
	m[path[len(path)-1]] = value
	return nil
}

func singleDocumentMap(v any) (map[string]any, bool) {
	switch value := v.(type) {
	case map[string]any:
		return value, true
	case []any:
		if len(value) != 1 {
			return nil, false
		}
		m, ok := value[0].(map[string]any)
		return m, ok
	default:
		return nil, false
	}
}
