package model

import (
	"encoding/json"
	"os"
	"testing"
)

type traceOperation struct {
	Kind  string `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

type traceSnapshot struct {
	Revision uint64 `json:"revision"`
	Text     string `json:"text"`
	Marks    []struct {
		ID    uint64 `json:"id"`
		Start int    `json:"start"`
		End   int    `json:"end"`
		Level uint8  `json:"level"`
	} `json:"marks"`
}

type traceCase struct {
	Initial    string           `json:"initial"`
	Name       string           `json:"name"`
	Operations []traceOperation `json:"operations"`
	Snapshots  []traceSnapshot  `json:"snapshots"`
}

func TestRustSeededDocumentTraces(t *testing.T) {
	data, err := os.ReadFile("../../fixtures/rust-oracle/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name  string      `json:"fixture"`
		Cases []traceCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var cases []traceCase
	for _, fixture := range fixtures {
		if fixture.Name == "seeded-traces-v1" {
			cases = fixture.Cases
			break
		}
	}
	if len(cases) == 0 {
		t.Fatal("seeded-traces-v1 missing")
	}
	for _, trace := range cases {
		t.Run(trace.Name, func(t *testing.T) {
			document, err := NewDocument("text")
			if err != nil {
				t.Fatal(err)
			}
			node := document.Snapshot().Root()
			if _, err := document.Transact(func(draft *DocumentDraft) error {
				return draft.SetTextValue(node, trace.Initial)
			}); err != nil {
				t.Fatal(err)
			}
			for index, operation := range trace.Operations {
				_, err := document.Transact(func(draft *DocumentDraft) error {
					switch operation.Kind {
					case "append":
						return draft.AppendText(node, operation.Text)
					case "replace":
						return draft.ReplaceTextUTF16(
							node,
							UTF16Offset(operation.Start),
							UTF16Offset(operation.End),
							operation.Text,
						)
					case "delete_prefix":
						return draft.ReplaceTextUTF16(node, 0, UTF16Offset(operation.Start), "")
					case "format":
						return draft.FormatEmphasis(
							node,
							NewUTF16Range(UTF16Offset(operation.Start), UTF16Offset(operation.End)),
							uint8(traceSeed(trace.Name)%2+1),
						)
					case "select":
						return nil
					default:
						return nil
					}
				})
				if err != nil {
					t.Fatalf("operation %d %#v: %v", index, operation, err)
				}
				got := snapshotForTrace(document.Snapshot(), node)
				want := trace.Snapshots[index]
				if got.Revision != want.Revision || got.Text != want.Text ||
					len(got.Marks) != len(want.Marks) {
					t.Fatalf("snapshot %d = %#v, want %#v", index, got, want)
				}
				for markIndex, mark := range got.Marks {
					expected := want.Marks[markIndex]
					if mark.ID != expected.ID || mark.Start != expected.Start ||
						mark.End != expected.End || mark.Level != expected.Level {
						t.Fatalf("snapshot %d mark %d = %#v, want %#v", index, markIndex, mark, expected)
					}
				}
			}
		})
	}
}

func snapshotForTrace(snapshot DocumentSnapshot, node NodeID) traceSnapshot {
	value, _ := snapshot.Node(node)
	result := traceSnapshot{Revision: snapshot.Revision(), Text: value.Text().String()}
	for _, mark := range value.Marks() {
		result.Marks = append(result.Marks, struct {
			ID    uint64 `json:"id"`
			Start int    `json:"start"`
			End   int    `json:"end"`
			Level uint8  `json:"level"`
		}{
			ID: uint64(mark.ID()), Start: int(mark.Range().Start),
			End: int(mark.Range().End), Level: mark.Level(),
		})
	}
	return result
}

func traceSeed(name string) uint64 {
	var seed uint64
	for _, r := range name {
		seed = seed*131 + uint64(r)
	}
	return seed
}
