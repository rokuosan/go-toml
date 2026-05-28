package toml

func (p *parser) parseTable() error {
	array := p.consume("[[")
	if !array && !p.consume("[") {
		return p.err("expected table")
	}
	p.skipWS()
	key, err := p.parseKey()
	if err != nil {
		return err
	}
	p.skipWS()
	if array {
		if !p.consume("]]") {
			return p.err("expected ]]")
		}
		m, err := p.appendArrayTable(key)
		if err != nil {
			return err
		}
		p.current = m
		return nil
	}
	if !p.consume("]") {
		return p.err("expected ]")
	}
	path := tablePath(key)
	if p.explicitTables[path] {
		return p.err("table already defined")
	}
	m, err := p.ensureTable(key)
	if err != nil {
		return err
	}
	p.explicitTables[path] = true
	p.current = m
	return nil
}
func (p *parser) parseInlineTable() (map[string]any, error) {
	if !p.consume("{") {
		return nil, p.err("expected inline table")
	}
	m := map[string]any{}
	p.skipWS()
	if p.consume("}") {
		return m, nil
	}
	for {
		if err := p.parseKeyValueInto(m); err != nil {
			return nil, err
		}
		p.skipWS()
		if p.consume("}") {
			return m, nil
		}
		if !p.consume(",") {
			return nil, p.err("expected , or }")
		}
		p.skipWS()
	}
}
func (p *parser) setValue(dst map[string]any, key []string, val any) error {
	m := dst
	for _, part := range key[:len(key)-1] {
		existing, ok := m[part]
		if !ok {
			child := map[string]any{}
			m[part] = child
			m = child
			continue
		}
		child, ok := existing.(map[string]any)
		if !ok {
			return p.err("key conflicts with existing value")
		}
		m = child
	}
	last := key[len(key)-1]
	if _, ok := m[last]; ok {
		return p.err("key already defined")
	}
	m[last] = val
	return nil
}

func (p *parser) ensureTable(key []string) (map[string]any, error) {
	m := map[string]any(p.doc)
	for _, part := range key {
		existing, ok := m[part]
		if !ok {
			child := map[string]any{}
			m[part] = child
			m = child
			continue
		}
		switch v := existing.(type) {
		case map[string]any:
			m = v
		case []any:
			if len(v) == 0 {
				return nil, p.err("empty array table")
			}
			child, ok := v[len(v)-1].(map[string]any)
			if !ok {
				return nil, p.err("array table contains non-table")
			}
			m = child
		default:
			return nil, p.err("table conflicts with existing value")
		}
	}
	return m, nil
}

func (p *parser) appendArrayTable(key []string) (map[string]any, error) {
	parent, err := p.ensureTable(key[:len(key)-1])
	if err != nil {
		return nil, err
	}
	last := key[len(key)-1]
	existing, ok := parent[last]
	if !ok {
		child := map[string]any{}
		parent[last] = []any{child}
		return child, nil
	}
	arr, ok := existing.([]any)
	if !ok {
		return nil, p.err("array table conflicts with existing value")
	}
	child := map[string]any{}
	parent[last] = append(arr, child)
	return child, nil
}
