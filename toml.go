package toml

import (
	"encoding"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Document is a parsed TOML document.
type Document map[string]any

// LocalDateTime is a TOML local date-time value.
type LocalDateTime struct {
	time.Time
}

// LocalDate is a TOML local date value.
type LocalDate struct {
	time.Time
}

// LocalTime is a TOML local time value.
type LocalTime struct {
	time.Duration
}

// Parse parses a TOML document.
func Parse(data []byte) (Document, error) {
	p := newParser(string(data))
	return p.parse()
}

// ParseString parses a TOML document.
func ParseString(s string) (Document, error) {
	p := newParser(s)
	return p.parse()
}

// Unmarshal parses a TOML document and stores the result in v.
func Unmarshal(data []byte, v any) error {
	doc, err := Parse(data)
	if err != nil {
		return err
	}
	return assignValue(reflect.ValueOf(v), doc)
}

type parseError struct {
	input string
	pos   int
	msg   string
}

func (e *parseError) Error() string {
	line, col := lineCol(e.input, e.pos)
	return fmt.Sprintf("toml: %s at line %d, column %d", e.msg, line, col)
}

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
	path := strings.Join(key, ".")
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

func (p *parser) parseValue() (any, error) {
	if p.eof() {
		return nil, p.err("expected value")
	}
	if strings.HasPrefix(p.input[p.pos:], "\"\"\"") {
		return p.parseMultilineBasicString()
	}
	if strings.HasPrefix(p.input[p.pos:], "'''") {
		return p.parseMultilineLiteralString()
	}
	switch p.peek() {
	case '"':
		return p.parseBasicString()
	case '\'':
		return p.parseLiteralString()
	case '[':
		return p.parseArray()
	case '{':
		return p.parseInlineTable()
	}

	token := p.readBareToken()
	if token == "" {
		return nil, p.err("expected value")
	}
	switch token {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "inf", "+inf":
		return math.Inf(1), nil
	case "-inf":
		return math.Inf(-1), nil
	case "nan", "+nan", "-nan":
		return math.NaN(), nil
	}
	if v, ok, err := parseDateTime(token); ok || err != nil {
		return v, err
	}
	if looksBasedInteger(token) {
		v, err := parseInteger(token)
		if err != nil {
			return nil, p.err("invalid value")
		}
		return v, nil
	}
	if looksFloat(token) {
		if !validFloat.MatchString(token) {
			return nil, p.err("invalid float")
		}
		v, err := strconv.ParseFloat(strings.ReplaceAll(token, "_", ""), 64)
		if err != nil {
			return nil, p.err("invalid float")
		}
		return v, nil
	}
	v, err := parseInteger(token)
	if err != nil {
		return nil, p.err("invalid value")
	}
	return v, nil
}

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

var validFloat = regexp.MustCompile(`^[+-]?(?:0|[1-9](?:[0-9]|_[0-9])*)(?:(?:\.[0-9](?:[0-9]|_[0-9])*(?:[eE][+-]?[0-9](?:[0-9]|_[0-9])*)?)|(?:[eE][+-]?[0-9](?:[0-9]|_[0-9])*))$`)

func parseInteger(token string) (int64, error) {
	sign := int64(1)
	if strings.HasPrefix(token, "+") {
		token = token[1:]
	} else if strings.HasPrefix(token, "-") {
		sign = -1
		token = token[1:]
	}
	base := 10
	switch {
	case strings.HasPrefix(token, "0x"):
		base = 16
		token = token[2:]
	case strings.HasPrefix(token, "0o"):
		base = 8
		token = token[2:]
	case strings.HasPrefix(token, "0b"):
		base = 2
		token = token[2:]
	}
	if token == "" || strings.HasPrefix(token, "_") || strings.HasSuffix(token, "_") || strings.Contains(token, "__") {
		return 0, fmt.Errorf("invalid integer")
	}
	clean := strings.ReplaceAll(token, "_", "")
	if base == 10 && len(clean) > 1 && clean[0] == '0' {
		return 0, fmt.Errorf("leading zero")
	}
	u, err := strconv.ParseUint(clean, base, 63)
	if err != nil {
		return 0, err
	}
	return int64(u) * sign, nil
}

func parseDateTime(token string) (any, bool, error) {
	if t, err := time.Parse(time.RFC3339Nano, normalizeTimeDelim(token)); err == nil {
		return t, true, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05.999999999", normalizeTimeDelim(token)); err == nil {
		return LocalDateTime{Time: t}, true, nil
	}
	if t, err := time.Parse("2006-01-02", token); err == nil {
		return LocalDate{Time: t}, true, nil
	}
	if d, ok := parseLocalTime(token); ok {
		return LocalTime{Duration: d}, true, nil
	}
	if looksDateTime(token) {
		return nil, true, fmt.Errorf("invalid date-time")
	}
	return nil, false, nil
}

func parseLocalTime(token string) (time.Duration, bool) {
	layouts := []string{"15:04:05.999999999", "15:04:05"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, token)
		if err == nil {
			return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second + time.Duration(t.Nanosecond()), true
		}
	}
	return 0, false
}

func normalizeTimeDelim(s string) string {
	if len(s) > 10 && (s[10] == ' ' || s[10] == 't') {
		return s[:10] + "T" + s[11:]
	}
	return s
}

func looksFloat(s string) bool {
	return strings.ContainsAny(s, ".eE")
}

func looksBasedInteger(s string) bool {
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	return strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0b")
}

func looksDateTime(s string) bool {
	return len(s) >= 10 && ((s[4] == '-' && s[7] == '-') || strings.Count(s, ":") >= 2)
}

func looksFullDate(s string) bool {
	return len(s) == len("2006-01-02") &&
		isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2]) && isDigit(s[3]) &&
		s[4] == '-' &&
		isDigit(s[5]) && isDigit(s[6]) &&
		s[7] == '-' &&
		isDigit(s[8]) && isDigit(s[9])
}

func countRun(s string, c byte) int {
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func lineCol(s string, pos int) (int, int) {
	line, col := 1, 1
	for i := 0; i < len(s) && i < pos; {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
		i += size
	}
	return line, col
}

func assignValue(dst reflect.Value, src any) error {
	if !dst.IsValid() {
		return fmt.Errorf("toml: invalid destination")
	}
	if dst.Kind() != reflect.Pointer || dst.IsNil() {
		return fmt.Errorf("toml: destination must be a non-nil pointer")
	}
	return assign(dst.Elem(), src)
}

func assign(dst reflect.Value, src any) error {
	if !dst.CanSet() {
		return nil
	}
	if src == nil {
		return nil
	}
	if tm, ok := dst.Addr().Interface().(encoding.TextUnmarshaler); ok {
		return tm.UnmarshalText([]byte(fmt.Sprint(src)))
	}
	sv := reflect.ValueOf(src)
	if sv.IsValid() && sv.Type().AssignableTo(dst.Type()) {
		dst.Set(sv)
		return nil
	}
	if sv.IsValid() && sv.Type().ConvertibleTo(dst.Type()) {
		dst.Set(sv.Convert(dst.Type()))
		return nil
	}
	switch dst.Kind() {
	case reflect.Pointer:
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return assign(dst.Elem(), src)
	case reflect.Struct:
		m, ok := asStringMap(src)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to %s", src, dst.Type())
		}
		return assignStruct(dst, m)
	case reflect.Map:
		m, ok := asStringMap(src)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to %s", src, dst.Type())
		}
		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}
		for k, v := range m {
			key := reflect.ValueOf(k)
			if !key.Type().AssignableTo(dst.Type().Key()) {
				if !key.Type().ConvertibleTo(dst.Type().Key()) {
					return fmt.Errorf("toml: cannot use string as map key %s", dst.Type().Key())
				}
				key = key.Convert(dst.Type().Key())
			}
			elem := reflect.New(dst.Type().Elem()).Elem()
			if err := assign(elem, v); err != nil {
				return err
			}
			dst.SetMapIndex(key, elem)
		}
		return nil
	case reflect.Slice:
		arr, ok := src.([]any)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to %s", src, dst.Type())
		}
		out := reflect.MakeSlice(dst.Type(), len(arr), len(arr))
		for i, v := range arr {
			if err := assign(out.Index(i), v); err != nil {
				return err
			}
		}
		dst.Set(out)
		return nil
	case reflect.String:
		s, ok := src.(string)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to string", src)
		}
		dst.SetString(s)
		return nil
	case reflect.Bool:
		b, ok := src.(bool)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to bool", src)
		}
		dst.SetBool(b)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, ok := src.(int64)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to int", src)
		}
		dst.SetInt(i)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, ok := src.(int64)
		if !ok || i < 0 {
			return fmt.Errorf("toml: cannot assign %T to uint", src)
		}
		dst.SetUint(uint64(i))
		return nil
	case reflect.Float32, reflect.Float64:
		f, ok := src.(float64)
		if !ok {
			return fmt.Errorf("toml: cannot assign %T to float", src)
		}
		dst.SetFloat(f)
		return nil
	case reflect.Interface:
		dst.Set(reflect.ValueOf(src))
		return nil
	default:
		return fmt.Errorf("toml: unsupported destination type %s", dst.Type())
	}
}

func asStringMap(src any) (map[string]any, bool) {
	switch v := src.(type) {
	case map[string]any:
		return v, true
	case Document:
		return map[string]any(v), true
	default:
		return nil, false
	}
}

func assignStruct(dst reflect.Value, src map[string]any) error {
	fields := map[string]int{}
	t := dst.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := f.Tag.Get("toml")
		if comma := strings.IndexByte(name, ','); comma >= 0 {
			name = name[:comma]
		}
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = i
		fields[strings.ToLower(name)] = i
	}
	for k, v := range src {
		i, ok := fields[k]
		if !ok {
			i, ok = fields[strings.ToLower(k)]
		}
		if !ok {
			continue
		}
		if err := assign(dst.Field(i), v); err != nil {
			return err
		}
	}
	return nil
}
