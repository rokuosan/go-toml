package toml

import (
	"strings"
	"unicode/utf8"
)

func (p *parser) skipWS() {
	for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
		p.pos++
	}
}

func (p *parser) skipArrayWS() error {
	for !p.eof() {
		p.skipWS()
		if p.eof() {
			return nil
		}
		if p.peek() == '#' {
			if err := p.skipComment(); err != nil {
				return err
			}
			continue
		}
		if p.consume("\r\n") || p.consume("\n") {
			continue
		}
		return nil
	}
	return nil
}

func (p *parser) skipComment() error {
	if !p.consume("#") {
		return p.err("expected comment")
	}
	for !p.eof() {
		c := p.peek()
		if c == '\n' || c == '\r' {
			return nil
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r < 0x20 && r != '\t' {
			return p.err("control character in comment")
		}
		p.pos += size
	}
	return nil
}

func (p *parser) consume(s string) bool {
	if strings.HasPrefix(p.input[p.pos:], s) {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *parser) peek() byte {
	return p.input[p.pos]
}

func (p *parser) eof() bool {
	return p.pos >= len(p.input)
}

func (p *parser) err(msg string) error {
	return p.errAt(p.pos, msg)
}

func (p *parser) errAt(pos int, msg string) error {
	return &parseError{input: p.input, pos: pos, msg: msg}
}
func (p *parser) readBareToken() string {
	start := p.pos
	for !p.eof() {
		c := p.peek()
		if c == ' ' && p.pos-start == len("2006-01-02") && looksFullDate(p.input[start:p.pos]) && p.pos+1 < len(p.input) && isDigit(p.input[p.pos+1]) {
			p.pos++
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '#' || c == ',' || c == ']' || c == '}' {
			break
		}
		p.pos++
	}
	return p.input[start:p.pos]
}
