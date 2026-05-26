package toml

import "unicode/utf8"

type parser struct {
	input          string
	pos            int
	doc            Document
	current        map[string]any
	explicitTables map[string]bool
}

func newParser(input string) *parser {
	doc := Document{}
	return &parser{
		input:          input,
		doc:            doc,
		current:        map[string]any(doc),
		explicitTables: map[string]bool{"": true},
	}
}

func (p *parser) parse() (Document, error) {
	if !utf8.ValidString(p.input) {
		return nil, p.errAt(0, "input is not valid UTF-8")
	}
	for !p.eof() {
		p.skipWS()
		if p.eof() {
			break
		}
		switch p.peek() {
		case '\n':
			p.pos++
			continue
		case '\r':
			if !p.consume("\r\n") {
				return nil, p.err("bare carriage return")
			}
			continue
		case '#':
			if err := p.skipComment(); err != nil {
				return nil, err
			}
			continue
		case '[':
			if err := p.parseTable(); err != nil {
				return nil, err
			}
		default:
			if err := p.parseKeyValueInto(p.current); err != nil {
				return nil, err
			}
		}
		p.skipWS()
		if p.eof() {
			break
		}
		if p.peek() == '#' {
			if err := p.skipComment(); err != nil {
				return nil, err
			}
		}
		if p.eof() {
			break
		}
		if p.consume("\r\n") || p.consume("\n") {
			continue
		}
		return nil, p.err("expected newline")
	}
	return p.doc, nil
}
