package toml

import (
	"reflect"
	"time"
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
