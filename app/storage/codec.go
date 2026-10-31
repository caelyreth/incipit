package storage

import (
	"encoding/json"
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

// A node's lists and maps are kept as compact JSON: readable with any
// SQLite tool, so a notebook can always be read without Neverland.

// storedValue is one attribute value, its kind named by which field is set.
type storedValue struct {
	Bool    *bool   `json:"b,omitempty"`
	Integer *int64  `json:"i,omitempty"`
	Text    *string `json:"t,omitempty"`
}

func encodeAttributes(attributes map[string]model.AttributeValue) (string, error) {
	stored := make(map[string]storedValue, len(attributes))
	for key, value := range attributes {
		var entry storedValue
		switch value.Kind() {
		case model.AttributeBool:
			boolean, _ := value.Bool()
			entry.Bool = &boolean
		case model.AttributeInteger:
			integer, _ := value.Integer()
			entry.Integer = &integer
		case model.AttributeText:
			text, _ := value.Text()
			entry.Text = &text
		}
		stored[key] = entry
	}
	encoded, err := json.Marshal(stored)
	return string(encoded), err
}

func decodeAttributes(encoded string) (map[string]model.AttributeValue, error) {
	var stored map[string]storedValue
	if err := json.Unmarshal([]byte(encoded), &stored); err != nil {
		return nil, err
	}
	attributes := make(map[string]model.AttributeValue, len(stored))
	for key, entry := range stored {
		switch {
		case entry.Bool != nil:
			attributes[key] = model.BoolValue(*entry.Bool)
		case entry.Integer != nil:
			attributes[key] = model.IntegerValue(*entry.Integer)
		case entry.Text != nil:
			attributes[key] = model.TextValue(*entry.Text)
		default:
			attributes[key] = model.NullValue()
		}
	}
	return attributes, nil
}

// storedMark is a mark as [identity, start, end, kind, level], with its
// range in UTF-16 code units as the model counts them.
type storedMark struct {
	ID    uint64
	Start uint64
	End   uint64
	Kind  string
	Level uint8
}

func (m storedMark) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{m.ID, m.Start, m.End, m.Kind, m.Level})
}

func (m *storedMark) UnmarshalJSON(data []byte) error {
	fields := []any{&m.ID, &m.Start, &m.End, &m.Kind, &m.Level}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != len(fields) {
		return fmt.Errorf("storage: a mark has %d fields, not %d", len(raw), len(fields))
	}
	for index, field := range fields {
		if err := json.Unmarshal(raw[index], field); err != nil {
			return err
		}
	}
	return nil
}

func encodeMarks(marks []model.InlineMark) (string, error) {
	stored := make([]storedMark, 0, len(marks))
	for _, mark := range marks {
		stored = append(stored, storedMark{
			ID:    mark.ID().Value(),
			Start: uint64(mark.Range().Start),
			End:   uint64(mark.Range().End),
			Kind:  string(mark.Kind()),
			Level: mark.Level(),
		})
	}
	encoded, err := json.Marshal(stored)
	return string(encoded), err
}

func decodeMarks(encoded string) ([]model.InlineMark, error) {
	var stored []storedMark
	if err := json.Unmarshal([]byte(encoded), &stored); err != nil {
		return nil, err
	}
	marks := make([]model.InlineMark, 0, len(stored))
	for _, mark := range stored {
		marks = append(marks, model.NewInlineMark(
			model.MarkID(mark.ID),
			model.NewUTF16Range(model.UTF16Offset(mark.Start), model.UTF16Offset(mark.End)),
			model.MarkKind(mark.Kind),
			mark.Level,
		))
	}
	return marks, nil
}

func encodeChildren(children []model.NodeID) (string, error) {
	if children == nil {
		children = []model.NodeID{}
	}
	encoded, err := json.Marshal(children)
	return string(encoded), err
}

func decodeChildren(encoded string) ([]model.NodeID, error) {
	var children []model.NodeID
	err := json.Unmarshal([]byte(encoded), &children)
	return children, err
}
