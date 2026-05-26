package toml

import (
	"reflect"
	"strings"
)

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

func tagName(f reflect.StructField) string {
	name := f.Tag.Get("toml")
	if comma := strings.IndexByte(name, ','); comma >= 0 {
		name = name[:comma]
	}
	if name == "" {
		name = f.Name
	}
	return name
}

func foldName(name string) string {
	return strings.ToLower(name)
}
