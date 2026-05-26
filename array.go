package toml

func (p *parser) parseArray() ([]any, error) {
	if !p.consume("[") {
		return nil, p.err("expected array")
	}
	var values []any
	for {
		if err := p.skipArrayWS(); err != nil {
			return nil, err
		}
		if p.consume("]") {
			return values, nil
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
		if err := p.skipArrayWS(); err != nil {
			return nil, err
		}
		if p.consume(",") {
			continue
		}
		if p.consume("]") {
			return values, nil
		}
		return nil, p.err("expected , or ]")
	}
}
