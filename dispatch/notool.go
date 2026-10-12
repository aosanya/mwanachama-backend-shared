package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type plainOperation Operation

func (op *Operation) UnmarshalJSON(raw []byte) error {
	var shadow struct {
		plainOperation
		Tool json.RawMessage `json:"tool,omitempty"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		return err
	}
	*op = Operation(shadow.plainOperation)
	op.Tool, op.NoTool = "", false
	tool := bytes.TrimSpace(shadow.Tool)
	switch {
	case len(tool) == 0, bytes.Equal(tool, []byte("null")):
	case bytes.Equal(tool, []byte("false")):
		op.NoTool = true
	case bytes.Equal(tool, []byte("true")):
		return fmt.Errorf(`"tool": true says nothing an omitted tool does not; name the tool or set it false`)
	default:
		if err := json.Unmarshal(tool, &op.Tool); err != nil {
			return fmt.Errorf(`"tool" is a tool name or false: %w`, err)
		}
	}
	return nil
}

func (op Operation) MarshalJSON() ([]byte, error) {
	if !op.NoTool {
		return json.Marshal(plainOperation(op))
	}
	return json.Marshal(struct {
		plainOperation
		Tool bool `json:"tool"`
	}{plainOperation(op), false})
}
