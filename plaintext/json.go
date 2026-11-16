package plaintext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type jsonFrame struct {
	object    bool
	expectKey bool
	key       string
	index     int
}

func CheckJSONStrings(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var stack []jsonFrame
	for {
		token, err := dec.Token()
		if err != nil {
			if bytes.IndexByte(raw, 0) >= 0 {
				return controlRefusal(jsonPath(stack))
			}
			return nil
		}
		switch value := token.(type) {
		case json.Delim:
			if value == '{' || value == '[' {
				stack = append(stack, jsonFrame{object: value == '{', expectKey: value == '{'})
				continue
			}
			stack = stack[:len(stack)-1]
		case string:
			if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].expectKey {
				stack[n-1].key = value
				stack[n-1].expectKey = false
				if hasForbiddenControl(value) {
					return controlRefusal(jsonPath(stack))
				}
				continue
			}
			if hasForbiddenControl(value) {
				return controlRefusal(jsonPath(stack))
			}
		}
		if len(stack) == 0 {
			return nil
		}
		top := &stack[len(stack)-1]
		if top.object {
			top.expectKey = true
		} else {
			top.index++
		}
	}
}
func hasForbiddenControl(text string) bool {
	return strings.IndexFunc(text, isControlOtherThanLayout) >= 0
}
func controlRefusal(field string) error {
	if field == "" {
		field = "the request body"
	}
	return fmt.Errorf("%w: %s holds a NUL byte or another control character; only line breaks and tabs are allowed", ErrNotPlainText, field)
}
func jsonPath(stack []jsonFrame) string {
	var path strings.Builder
	for _, frame := range stack {
		switch {
		case frame.object && frame.key != "":
			if path.Len() > 0 {
				path.WriteByte('.')
			}
			path.WriteString(frame.key)
		case !frame.object:
			fmt.Fprintf(&path, "[%d]", frame.index)
		}
	}
	return path.String()
}
