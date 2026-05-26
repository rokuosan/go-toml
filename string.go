package toml

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

func (p *parser) parseBasicString() (string, error) {
	if !p.consume("\"") {
		return "", p.err("expected string")
	}
	var b strings.Builder
	for !p.eof() {
		c := p.peek()
		if c == '"' {
			p.pos++
			return b.String(), nil
		}
		if c == '\n' || c == '\r' {
			return "", p.err("newline in basic string")
		}
		if c == '\\' {
			r, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
			continue
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r < 0x20 && r != '\t' {
			return "", p.err("control character in string")
		}
		b.WriteRune(r)
		p.pos += size
	}
	return "", p.err("unterminated string")
}

func (p *parser) parseMultilineBasicString() (string, error) {
	p.pos += 3
	p.consume("\r\n")
	p.consume("\n")
	var b strings.Builder
	for !p.eof() {
		if strings.HasPrefix(p.input[p.pos:], "\"\"\"") {
			quotes := countRun(p.input[p.pos:], '"')
			if quotes <= 5 {
				extra := quotes - 3
				b.WriteString(strings.Repeat("\"", extra))
				p.pos += quotes
				return b.String(), nil
			}
		}
		if p.peek() == '\\' {
			if p.consumeEscapedNewline() {
				continue
			}
			r, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
			continue
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return "", p.err("control character in string")
		}
		b.WriteRune(r)
		p.pos += size
	}
	return "", p.err("unterminated multiline string")
}

func (p *parser) parseLiteralString() (string, error) {
	if !p.consume("'") {
		return "", p.err("expected string")
	}
	start := p.pos
	for !p.eof() {
		c := p.peek()
		if c == '\'' {
			s := p.input[start:p.pos]
			p.pos++
			return s, nil
		}
		if c == '\n' || c == '\r' {
			return "", p.err("newline in literal string")
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r < 0x20 && r != '\t' {
			return "", p.err("control character in string")
		}
		p.pos += size
	}
	return "", p.err("unterminated string")
}

func (p *parser) parseMultilineLiteralString() (string, error) {
	p.pos += 3
	p.consume("\r\n")
	p.consume("\n")
	var b strings.Builder
	for !p.eof() {
		if strings.HasPrefix(p.input[p.pos:], "'''") {
			quotes := countRun(p.input[p.pos:], '\'')
			if quotes <= 5 {
				extra := quotes - 3
				b.WriteString(strings.Repeat("'", extra))
				p.pos += quotes
				return b.String(), nil
			}
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return "", p.err("control character in string")
		}
		b.WriteRune(r)
		p.pos += size
	}
	return "", p.err("unterminated multiline string")
}

func (p *parser) parseEscape() (rune, error) {
	if !p.consume("\\") || p.eof() {
		return 0, p.err("invalid escape")
	}
	c := p.peek()
	p.pos++
	switch c {
	case '"':
		return '"', nil
	case '\\':
		return '\\', nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		return p.parseHexRune(4)
	case 'U':
		return p.parseHexRune(8)
	default:
		return 0, p.err("invalid escape")
	}
}

func (p *parser) parseHexRune(n int) (rune, error) {
	if len(p.input)-p.pos < n {
		return 0, p.err("short unicode escape")
	}
	s := p.input[p.pos : p.pos+n]
	for _, c := range s {
		if !isHex(c) {
			return 0, p.err("invalid unicode escape")
		}
	}
	p.pos += n
	v, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0, p.err("invalid unicode escape")
	}
	r := rune(v)
	if !utf8.ValidRune(r) || (r >= 0xD800 && r <= 0xDFFF) {
		return 0, p.err("invalid unicode scalar")
	}
	return r, nil
}

func (p *parser) consumeEscapedNewline() bool {
	save := p.pos
	if !p.consume("\\") {
		return false
	}
	for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
		p.pos++
	}
	if !p.consume("\r\n") && !p.consume("\n") {
		p.pos = save
		return false
	}
	for {
		for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
			p.pos++
		}
		if !p.consume("\r\n") && !p.consume("\n") {
			break
		}
	}
	return true
}
