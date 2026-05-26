package toml

import (
	"encoding"
	"fmt"
	"reflect"
)

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
	fields := cachedFields(dst.Type())
	for k, v := range src {
		i, ok := fields[k]
		if !ok {
			i, ok = fields[foldName(k)]
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
