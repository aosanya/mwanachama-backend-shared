package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const unknownFieldPrefix = "json: unknown field "

func describeDecodeError(err error, fallbackField string) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = fallbackField
		}
		if field == "" {
			return fmt.Errorf("the value must be %s", expectedWord(typeErr.Type))
		}
		return fmt.Errorf("%s must be %s", field, expectedWord(typeErr.Type))
	}
	if quoted, ok := strings.CutPrefix(err.Error(), unknownFieldPrefix); ok {
		return fmt.Errorf("%s is not a field this request takes", strings.Trim(quoted, `"`))
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return errors.New("the body is not valid JSON")
	}
	if fallbackField != "" {
		return fmt.Errorf("%s is not a valid value", fallbackField)
	}
	return errors.New("the body could not be read")
}
func expectedWord(t reflect.Type) string {
	if t == nil {
		return "a different kind of value"
	}
	switch t.Kind() {
	case reflect.String:
		return "text"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a whole number"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "a list"
	case reflect.Struct, reflect.Map:
		return "an object"
	case reflect.Pointer:
		return expectedWord(t.Elem())
	}
	return "a different kind of value"
}
