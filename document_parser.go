package toml

import "unicode/utf8"

func (p *parser) parseSyntaxDocument() (*SyntaxDocument, error) {
	if !utf8.ValidString(p.input) {
		return nil, p.errAt(0, "input is not valid UTF-8")
	}
	doc := &SyntaxDocument{source: p.input, doc: p.doc}
	var currentPath []string
	for !p.eof() {
		lineStart := p.pos
		p.skipWS()
		if p.eof() {
			if p.pos > lineStart {
				doc.appendNode(SyntaxBlankLine, lineStart, p.pos, 0, 0, nil, nil)
			}
			break
		}
		switch p.peek() {
		case '\n':
			p.pos++
			doc.appendNode(SyntaxBlankLine, lineStart, p.pos, 0, 0, nil, nil)
			continue
		case '\r':
			if !p.consume("\r\n") {
				return nil, p.err("bare carriage return")
			}
			doc.appendNode(SyntaxBlankLine, lineStart, p.pos, 0, 0, nil, nil)
			continue
		case '#':
			if err := p.skipComment(); err != nil {
				return nil, err
			}
			if err := p.finishSyntaxLine(); err != nil {
				return nil, err
			}
			doc.appendNode(SyntaxComment, lineStart, p.pos, 0, 0, nil, nil)
			continue
		case '[':
			key, array, keyStart, keyEnd, err := p.parseSyntaxTable()
			if err != nil {
				return nil, err
			}
			if err := p.finishSyntaxLine(); err != nil {
				return nil, err
			}
			kind := SyntaxTable
			if array {
				kind = SyntaxArrayTable
			}
			currentPath = append([]string(nil), key...)
			doc.appendNode(kind, lineStart, p.pos, keyStart, keyEnd, key, currentPath)
		default:
			keyStart := p.pos
			kv, err := p.parseSyntaxKeyValue()
			if err != nil {
				return nil, err
			}
			if err := p.finishSyntaxLine(); err != nil {
				return nil, err
			}
			path := append(append([]string(nil), currentPath...), kv.key...)
			doc.appendKeyValueNode(lineStart, p.pos, keyStart, kv.keyEnd, kv.valueStart, kv.valueEnd, kv.key, path, kv.value)
		}
	}
	return doc, nil
}

type syntaxKeyValue struct {
	key        []string
	value      any
	keyEnd     int
	valueStart int
	valueEnd   int
}

func (d *SyntaxDocument) appendNode(kind SyntaxKind, start, end, keyStart, keyEnd int, key, path []string) {
	node := SyntaxNode{
		Kind:     kind,
		Start:    start,
		End:      end,
		KeyStart: keyStart,
		KeyEnd:   keyEnd,
		Key:      append([]string(nil), key...),
		Path:     append([]string(nil), path...),
		Raw:      d.source[start:end],
		RawKey:   d.source[keyStart:keyEnd],
	}
	d.nodes = append(d.nodes, node)
}

func (d *SyntaxDocument) appendKeyValueNode(start, end, keyStart, keyEnd, valueStart, valueEnd int, key, path []string, value any) {
	node := SyntaxNode{
		Kind:       SyntaxKeyValue,
		Start:      start,
		End:        end,
		KeyStart:   keyStart,
		KeyEnd:     keyEnd,
		ValueStart: valueStart,
		ValueEnd:   valueEnd,
		Key:        append([]string(nil), key...),
		Path:       append([]string(nil), path...),
		Value:      value,
		Raw:        d.source[start:end],
		RawKey:     d.source[keyStart:keyEnd],
		RawValue:   d.source[valueStart:valueEnd],
	}
	d.nodes = append(d.nodes, node)
}

func (p *parser) finishSyntaxLine() error {
	p.skipWS()
	if p.eof() {
		return nil
	}
	if p.peek() == '#' {
		if err := p.skipComment(); err != nil {
			return err
		}
	}
	if p.eof() {
		return nil
	}
	if p.consume("\r\n") || p.consume("\n") {
		return nil
	}
	return p.err("expected newline")
}

func (p *parser) parseSyntaxTable() ([]string, bool, int, int, error) {
	array := p.consume("[[")
	if !array && !p.consume("[") {
		return nil, false, 0, 0, p.err("expected table")
	}
	p.skipWS()
	keyStart := p.pos
	key, err := p.parseKey()
	if err != nil {
		return nil, false, 0, 0, err
	}
	keyEnd := p.pos
	p.skipWS()
	if array {
		if !p.consume("]]") {
			return nil, false, 0, 0, p.err("expected ]]")
		}
		m, err := p.appendArrayTable(key)
		if err != nil {
			return nil, false, 0, 0, err
		}
		p.current = m
		return key, true, keyStart, keyEnd, nil
	}
	if !p.consume("]") {
		return nil, false, 0, 0, p.err("expected ]")
	}
	path := tablePath(key)
	if p.explicitTables[path] {
		return nil, false, 0, 0, p.err("table already defined")
	}
	m, err := p.ensureTable(key)
	if err != nil {
		return nil, false, 0, 0, err
	}
	p.explicitTables[path] = true
	p.current = m
	return key, false, keyStart, keyEnd, nil
}

func (p *parser) parseSyntaxKeyValue() (syntaxKeyValue, error) {
	key, err := p.parseKey()
	if err != nil {
		return syntaxKeyValue{}, err
	}
	keyEnd := p.pos
	p.skipWS()
	if !p.consume("=") {
		return syntaxKeyValue{}, p.err("expected =")
	}
	p.skipWS()
	valueStart := p.pos
	val, err := p.parseValue()
	if err != nil {
		return syntaxKeyValue{}, err
	}
	valueEnd := p.pos
	if err := p.setValue(p.current, key, val); err != nil {
		return syntaxKeyValue{}, err
	}
	return syntaxKeyValue{
		key:        key,
		value:      val,
		keyEnd:     keyEnd,
		valueStart: valueStart,
		valueEnd:   valueEnd,
	}, nil
}
