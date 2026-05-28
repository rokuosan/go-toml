package toml

import "unicode/utf8"

func (p *parser) parseSyntaxDocument() (*SyntaxDocument, error) {
	if !utf8.ValidString(p.input) {
		return nil, p.errAt(0, "input is not valid UTF-8")
	}
	doc := &SyntaxDocument{source: p.input, doc: p.doc}
	for !p.eof() {
		lineStart := p.pos
		p.skipWS()
		if p.eof() {
			if p.pos > lineStart {
				doc.appendNode(SyntaxBlankLine, lineStart, p.pos, 0, 0, nil)
			}
			break
		}
		switch p.peek() {
		case '\n':
			p.pos++
			doc.appendNode(SyntaxBlankLine, lineStart, p.pos, 0, 0, nil)
			continue
		case '\r':
			if !p.consume("\r\n") {
				return nil, p.err("bare carriage return")
			}
			doc.appendNode(SyntaxBlankLine, lineStart, p.pos, 0, 0, nil)
			continue
		case '#':
			if err := p.skipComment(); err != nil {
				return nil, err
			}
			if err := p.finishSyntaxLine(); err != nil {
				return nil, err
			}
			doc.appendNode(SyntaxComment, lineStart, p.pos, 0, 0, nil)
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
			doc.appendNode(kind, lineStart, p.pos, keyStart, keyEnd, key)
		default:
			keyStart := p.pos
			key, keyEnd, valueStart, valueEnd, err := p.parseSyntaxKeyValue()
			if err != nil {
				return nil, err
			}
			if err := p.finishSyntaxLine(); err != nil {
				return nil, err
			}
			doc.appendKeyValueNode(lineStart, p.pos, keyStart, keyEnd, valueStart, valueEnd, key)
		}
	}
	return doc, nil
}

func (d *SyntaxDocument) appendNode(kind SyntaxKind, start, end, keyStart, keyEnd int, key []string) {
	node := SyntaxNode{
		Kind:     kind,
		Start:    start,
		End:      end,
		KeyStart: keyStart,
		KeyEnd:   keyEnd,
		Key:      append([]string(nil), key...),
		Raw:      d.source[start:end],
		RawKey:   d.source[keyStart:keyEnd],
	}
	d.nodes = append(d.nodes, node)
}

func (d *SyntaxDocument) appendKeyValueNode(start, end, keyStart, keyEnd, valueStart, valueEnd int, key []string) {
	node := SyntaxNode{
		Kind:       SyntaxKeyValue,
		Start:      start,
		End:        end,
		KeyStart:   keyStart,
		KeyEnd:     keyEnd,
		ValueStart: valueStart,
		ValueEnd:   valueEnd,
		Key:        append([]string(nil), key...),
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

func (p *parser) parseSyntaxKeyValue() ([]string, int, int, int, error) {
	key, err := p.parseKey()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	keyEnd := p.pos
	p.skipWS()
	if !p.consume("=") {
		return nil, 0, 0, 0, p.err("expected =")
	}
	p.skipWS()
	valueStart := p.pos
	val, err := p.parseValue()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	valueEnd := p.pos
	if err := p.setValue(p.current, key, val); err != nil {
		return nil, 0, 0, 0, err
	}
	return key, keyEnd, valueStart, valueEnd, nil
}
