package toml

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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
