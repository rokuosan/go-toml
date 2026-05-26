package toml

func (p *parser) parseKeyValueInto(dst map[string]any) error {
	key, err := p.parseKey()
	if err != nil {
		return err
	}
	p.skipWS()
	if !p.consume("=") {
		return p.err("expected =")
	}
	p.skipWS()
	val, err := p.parseValue()
	if err != nil {
		return err
	}
	return p.setValue(dst, key, val)
}

func (p *parser) parseKey() ([]string, error) {
	part, err := p.parseSimpleKey()
	if err != nil {
		return nil, err
	}
	key := []string{part}
	for {
		save := p.pos
		p.skipWS()
		if !p.consume(".") {
			p.pos = save
			break
		}
		p.skipWS()
		part, err = p.parseSimpleKey()
		if err != nil {
			return nil, err
		}
		key = append(key, part)
	}
	return key, nil
}

func (p *parser) parseSimpleKey() (string, error) {
	if p.eof() {
		return "", p.err("expected key")
	}
	switch p.peek() {
	case '"':
		return p.parseBasicString()
	case '\'':
		return p.parseLiteralString()
	default:
		start := p.pos
		for !p.eof() {
			c := p.peek()
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
				p.pos++
				continue
			}
			break
		}
		if p.pos == start {
			return "", p.err("expected key")
		}
		return p.input[start:p.pos], nil
	}
}
