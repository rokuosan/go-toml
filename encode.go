package toml

import (
	"encoding"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Marshal returns the TOML encoding of v.
func Marshal(v any) ([]byte, error) {
	var enc encoder
	if err := enc.marshal(v); err != nil {
		return nil, err
	}
	return []byte(enc.b.String()), nil
}

type encoder struct {
	b strings.Builder
}

type encodeField struct {
	name  string
	value reflect.Value
}

func (e *encoder) marshal(v any) error {
	rv := indirectValue(reflect.ValueOf(v))
	if !rv.IsValid() {
		return fmt.Errorf("toml: cannot marshal nil")
	}
	if !isTableValue(rv) {
		return fmt.Errorf("toml: top-level value must be a struct or map")
	}
	return e.writeTable(nil, rv, false, false)
}

func (e *encoder) writeTable(path []string, v reflect.Value, header, array bool) error {
	v = indirectValue(v)
	if !v.IsValid() {
		return nil
	}
	if header {
		if e.b.Len() > 0 {
			e.b.WriteByte('\n')
		}
		if array {
			fmt.Fprintf(&e.b, "[[%s]]\n", strings.Join(path, "."))
		} else {
			fmt.Fprintf(&e.b, "[%s]\n", strings.Join(path, "."))
		}
	}

	fields, err := tableFields(v)
	if err != nil {
		return err
	}
	var nested []encodeField
	var arrayTables []encodeField
	for _, field := range fields {
		fv := indirectValue(field.value)
		if !fv.IsValid() {
			continue
		}
		if isArrayTableValue(fv) {
			arrayTables = append(arrayTables, field)
			continue
		}
		if isTableValue(fv) {
			nested = append(nested, field)
			continue
		}
		value, err := encodeScalarValue(fv)
		if err != nil {
			return fmt.Errorf("toml: field %s: %w", field.name, err)
		}
		fmt.Fprintf(&e.b, "%s = %s\n", quoteKey(field.name), value)
	}

	for _, field := range nested {
		if err := e.writeTable(append(path, quoteKey(field.name)), field.value, true, false); err != nil {
			return err
		}
	}
	for _, field := range arrayTables {
		fv := indirectValue(field.value)
		for i := 0; i < fv.Len(); i++ {
			if err := e.writeTable(append(path, quoteKey(field.name)), fv.Index(i), true, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func tableFields(v reflect.Value) ([]encodeField, error) {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		fields := make([]encodeField, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			if sf.PkgPath != "" {
				continue
			}
			name := tagName(sf)
			if name == "-" {
				continue
			}
			fields = append(fields, encodeField{name: name, value: v.Field(i)})
		}
		return fields, nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key must be string, got %s", v.Type().Key())
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		fields := make([]encodeField, 0, len(keys))
		for _, key := range keys {
			fields = append(fields, encodeField{name: key.String(), value: v.MapIndex(key)})
		}
		return fields, nil
	default:
		return nil, fmt.Errorf("expected table value, got %s", v.Type())
	}
}

func encodeScalarValue(v reflect.Value) (string, error) {
	v = indirectValue(v)
	if !v.IsValid() {
		return "", fmt.Errorf("nil has no TOML representation")
	}
	if v.Kind() == reflect.Struct {
		if value, err := encodeStructScalar(v); err == nil {
			return value, nil
		}
	}
	if text, ok, err := encodeTextMarshaler(v); ok || err != nil {
		if err != nil {
			return "", err
		}
		return quoteString(text), nil
	}
	switch v.Kind() {
	case reflect.String:
		return quoteString(v.String()), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Uint() > math.MaxInt64 {
			return "", fmt.Errorf("unsigned integer %d exceeds TOML integer range", v.Uint())
		}
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return encodeFloat(v.Float()), nil
	case reflect.Slice, reflect.Array:
		return encodeArray(v)
	case reflect.Struct:
		return "", fmt.Errorf("unsupported value type %s", v.Type())
	default:
		return "", fmt.Errorf("unsupported value type %s", v.Type())
	}
}

func encodeArray(v reflect.Value) (string, error) {
	values := make([]string, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem := indirectValue(v.Index(i))
		if !elem.IsValid() {
			return "", fmt.Errorf("array element %d is nil", i)
		}
		if isTableValue(elem) {
			value, err := encodeInlineTable(elem)
			if err != nil {
				return "", err
			}
			values = append(values, value)
			continue
		}
		value, err := encodeScalarValue(elem)
		if err != nil {
			return "", err
		}
		values = append(values, value)
	}
	return "[" + strings.Join(values, ", ") + "]", nil
}

func encodeInlineTable(v reflect.Value) (string, error) {
	fields, err := tableFields(v)
	if err != nil {
		return "", err
	}
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		fv := indirectValue(field.value)
		if !fv.IsValid() {
			continue
		}
		if isTableValue(fv) || isArrayTableValue(fv) {
			return "", fmt.Errorf("nested table is not supported in inline table")
		}
		value, err := encodeScalarValue(fv)
		if err != nil {
			return "", err
		}
		values = append(values, quoteKey(field.name)+" = "+value)
	}
	return "{ " + strings.Join(values, ", ") + " }", nil
}

func encodeStructScalar(v reflect.Value) (string, error) {
	if v.Type() == reflect.TypeOf(time.Time{}) {
		return v.Interface().(time.Time).Format(time.RFC3339Nano), nil
	}
	if v.Type() == reflect.TypeOf(LocalDateTime{}) {
		return v.Interface().(LocalDateTime).Format("2006-01-02T15:04:05.999999999"), nil
	}
	if v.Type() == reflect.TypeOf(LocalDate{}) {
		return v.Interface().(LocalDate).Format("2006-01-02"), nil
	}
	if v.Type() == reflect.TypeOf(LocalTime{}) {
		return formatLocalTime(v.Interface().(LocalTime).Duration), nil
	}
	return "", fmt.Errorf("unsupported value type %s", v.Type())
}

func encodeTextMarshaler(v reflect.Value) (string, bool, error) {
	if v.CanInterface() {
		if tm, ok := v.Interface().(encoding.TextMarshaler); ok {
			text, err := tm.MarshalText()
			return string(text), true, err
		}
	}
	if v.CanAddr() {
		if tm, ok := v.Addr().Interface().(encoding.TextMarshaler); ok {
			text, err := tm.MarshalText()
			return string(text), true, err
		}
	}
	return "", false, nil
}

func encodeFloat(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	case math.IsNaN(v):
		return "nan"
	default:
		s := strconv.FormatFloat(v, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	}
}

func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04X`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func quoteKey(s string) string {
	if s != "" && isBareKey(s) {
		return s
	}
	return quoteString(s)
}

func isBareKey(s string) bool {
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func formatLocalTime(d time.Duration) string {
	h := d / time.Hour
	d %= time.Hour
	m := d / time.Minute
	d %= time.Minute
	s := d / time.Second
	d %= time.Second
	if d == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return strings.TrimRight(fmt.Sprintf("%02d:%02d:%02d.%09d", h, m, s, d), "0")
}

func isTableValue(v reflect.Value) bool {
	v = indirectValue(v)
	if !v.IsValid() {
		return false
	}
	if isTextMarshalerValue(v) {
		return false
	}
	if _, err := encodeStructScalar(v); err == nil {
		return false
	}
	return v.Kind() == reflect.Struct || v.Kind() == reflect.Map
}

func isArrayTableValue(v reflect.Value) bool {
	v = indirectValue(v)
	if !v.IsValid() || (v.Kind() != reflect.Slice && v.Kind() != reflect.Array) {
		return false
	}
	if v.Len() == 0 {
		return false
	}
	for i := 0; i < v.Len(); i++ {
		if !isTableValue(v.Index(i)) {
			return false
		}
	}
	return true
}

func indirectValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func isTextMarshalerValue(v reflect.Value) bool {
	if v.CanInterface() {
		if _, ok := v.Interface().(encoding.TextMarshaler); ok {
			return true
		}
	}
	if v.CanAddr() {
		if _, ok := v.Addr().Interface().(encoding.TextMarshaler); ok {
			return true
		}
	}
	return false
}
