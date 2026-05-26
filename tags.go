package toml

import (
	"reflect"
	"strings"
)

// tagOptions is the string following a comma in a struct field's "toml"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

func cachedFields(t reflect.Type) map[string]int {
	fields := map[string]int{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := tagName(f)
		if name == "-" {
			continue
		}
		fields[name] = i
		fields[foldName(name)] = i
	}
	return fields
}

// parseTag splits a struct field's toml tag into its name and comma-separated
// options.
func parseTag(tag string) (string, tagOptions) {
	name, opt, _ := strings.Cut(tag, ",")
	return name, tagOptions(opt)
}

// Contains reports whether a comma-separated list of options contains
// optionName.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var name string
		name, s, _ = strings.Cut(s, ",")
		if name == optionName {
			return true
		}
	}
	return false
}

func tagName(f reflect.StructField) string {
	name, _ := parseTag(f.Tag.Get("toml"))
	if name == "" {
		name = f.Name
	}
	return name
}

func foldName(name string) string {
	return strings.ToLower(name)
}
