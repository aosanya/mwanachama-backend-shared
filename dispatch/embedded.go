package dispatch

import (
	"reflect"
	"strings"
)

type fieldOf struct {
	field    reflect.StructField
	owner    string
	embedded bool
}

func fieldsOf(t reflect.Type) []fieldOf {
	var out []fieldOf
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if inner, ok := embeddedStruct(f); ok {
			for _, at := range fieldsOf(inner) {
				at.embedded = true
				out = append(out, at)
			}
			continue
		}
		out = append(out, fieldOf{field: f, owner: t.Name()})
	}
	return out
}

func embeddedStruct(f reflect.StructField) (reflect.Type, bool) {
	if !f.Anonymous {
		return nil, false
	}
	if tag, ok := f.Tag.Lookup("json"); ok && strings.Split(tag, ",")[0] != "" {
		return nil, false
	}
	t := f.Type
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, t.Kind() == reflect.Struct
}
