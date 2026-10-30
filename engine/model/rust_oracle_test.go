package model

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type oracleCoordinateCase struct {
	Text     string `json:"text"`
	UTF16Len int    `json:"utf16_len"`
	ByteLen  int    `json:"byte_len"`
	Queries  []struct {
		Start   int `json:"start"`
		End     int `json:"end"`
		Aligned *struct {
			Start     int `json:"start"`
			End       int `json:"end"`
			ByteStart int `json:"byte_start"`
			ByteEnd   int `json:"byte_end"`
		} `json:"aligned"`
	} `json:"queries"`
	Clamped []struct {
		UTF16 int `json:"utf16"`
		Byte  int `json:"byte"`
	} `json:"clamped"`
	ByteStrict []struct {
		Byte  int  `json:"byte"`
		UTF16 *int `json:"utf16"`
	} `json:"byte_strict"`
}

type oracleAnchorCase struct {
	Edit struct {
		Start    int `json:"start"`
		End      int `json:"end"`
		Inserted int `json:"inserted"`
	} `json:"edit"`
	Offset   int    `json:"offset"`
	Affinity string `json:"affinity"`
	Result   *int   `json:"result"`
}

func loadOracle(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("../../fixtures/rust-oracle/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name string          `json:"fixture"`
		Raw  json.RawMessage `json:"-"`
	}
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]json.RawMessage)
	for _, value := range values {
		var name string
		if err := json.Unmarshal(value["fixture"], &name); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = raw
	}
	_ = fixtures
	return out
}

func TestRustOracleCoordinates(t *testing.T) {
	var fixture struct {
		Cases []oracleCoordinateCase `json:"cases"`
	}
	if err := json.Unmarshal(loadOracle(t)["coordinates-v1"], &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		text := test.Text
		if got := StringLenUTF16(test.Text); got != test.UTF16Len {
			t.Fatalf("%q utf16 len = %d, want %d", test.Text, got, test.UTF16Len)
		}
		if len(test.Text) != test.ByteLen {
			t.Fatalf("%q byte len = %d, want %d", test.Text, len(test.Text), test.ByteLen)
		}
		for _, query := range test.Queries {
			aligned, bytes, ok := AlignUTF16RangeForNativeQuery(
				test.Text,
				NewUTF16Range(UTF16Offset(query.Start), UTF16Offset(query.End)),
			)
			if query.Aligned == nil {
				if ok {
					t.Fatalf("%q %d..%d aligned unexpectedly: %#v %#v", test.Text, query.Start, query.End, aligned, bytes)
				}
				continue
			}
			if !ok ||
				int(aligned.Start) != query.Aligned.Start ||
				int(aligned.End) != query.Aligned.End ||
				int(bytes.Start) != query.Aligned.ByteStart ||
				int(bytes.End) != query.Aligned.ByteEnd {
				t.Fatalf("%q %d..%d = %#v %#v,%v, want %#v", test.Text, query.Start, query.End, aligned, bytes, ok, query.Aligned)
			}
		}
		for _, clamped := range test.Clamped {
			if got := UTF16OffsetToByteClamped(text, UTF16Offset(clamped.UTF16)); int(got) != clamped.Byte {
				t.Fatalf("%q clamped %d = %d, want %d", text, clamped.UTF16, got, clamped.Byte)
			}
		}
		for _, strict := range test.ByteStrict {
			got, ok := ByteOffsetToUTF16(text, ByteOffset(strict.Byte))
			if strict.UTF16 == nil {
				if ok {
					t.Fatalf("%q byte %d = %d, want invalid", text, strict.Byte, got)
				}
				continue
			}
			if !ok || int(got) != *strict.UTF16 {
				t.Fatalf("%q byte %d = %d,%v, want %d", text, strict.Byte, got, ok, *strict.UTF16)
			}
		}
	}
}

func TestRustOracleAnchorRebase(t *testing.T) {
	var fixture struct {
		Cases []oracleAnchorCase `json:"cases"`
	}
	if err := json.Unmarshal(loadOracle(t)["anchor-rebase-v1"], &fixture); err != nil {
		t.Fatal(err)
	}
	const node NodeID = 1
	for _, test := range fixture.Cases {
		edit := NewTextEdit(node, NewUTF16Range(
			UTF16Offset(test.Edit.Start),
			UTF16Offset(test.Edit.End),
		), UTF16Offset(test.Edit.Inserted))
		affinity := AffinityLeft
		if test.Affinity == "right" {
			affinity = AffinityRight
		}
		got, ok := NewTextAnchor(node, UTF16Offset(test.Offset), affinity, 0).Rebase(edit, 1)
		if test.Result == nil {
			if ok {
				t.Fatalf("edit %#v offset %d %s = %d, want invalid", test.Edit, test.Offset, test.Affinity, got.OffsetUTF16())
			}
			continue
		}
		if !ok || int(got.OffsetUTF16()) != *test.Result {
			t.Fatalf("edit %#v offset %d %s = %d,%v, want %d", test.Edit, test.Offset, test.Affinity, got.OffsetUTF16(), ok, *test.Result)
		}
	}
}

func TestRustOracleTextMetricsPrefix(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Text       string `json:"text"`
			Characters int    `json:"characters"`
			Paragraphs int    `json:"paragraphs"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(loadOracle(t)["text-metrics-v1"], &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		if got := StringLenRunes(test.Text); got != test.Characters {
			t.Fatalf("%q characters = %d, want %d", test.Text, got, test.Characters)
		}
		paragraphs := 0
		for _, line := range strings.Split(test.Text, "\n") {
			if strings.TrimSpace(line) != "" {
				paragraphs++
			}
		}
		if paragraphs != test.Paragraphs {
			t.Fatalf("%q paragraphs = %d, want %d", test.Text, paragraphs, test.Paragraphs)
		}
	}
}

func TestRustOracleMarks(t *testing.T) {
	var fixture struct {
		Initial []struct {
			ID    uint64 `json:"id"`
			Start int    `json:"start"`
			End   int    `json:"end"`
			Level uint8  `json:"level"`
		} `json:"initial_marks"`
		Final []struct {
			ID    uint64 `json:"id"`
			Start int    `json:"start"`
			End   int    `json:"end"`
			Level uint8  `json:"level"`
		} `json:"final_marks"`
		Text string `json:"text_after_insert"`
	}
	if err := json.Unmarshal(loadOracle(t)["marks-v1"], &fixture); err != nil {
		t.Fatal(err)
	}
	document, err := NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		return draft.SetTextValue(node, "abcdef")
	}); err != nil {
		t.Fatal(err)
	}
	var first, nested MarkID
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		var err error
		first, err = draft.AddMark(node, NewUTF16Range(1, 5), "emphasis", 1)
		if err != nil {
			return err
		}
		nested, err = draft.AddMark(node, NewUTF16Range(2, 4), "emphasis", 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		return draft.ReplaceTextUTF16(node, 0, 0, "界")
	}); err != nil {
		t.Fatal(err)
	}
	value, _ := document.Snapshot().Node(node)
	if value.Text().String() != fixture.Text {
		t.Fatalf("text = %q, want %q", value.Text().String(), fixture.Text)
	}
	got := value.Marks()
	if len(got) != len(fixture.Initial) {
		t.Fatalf("marks = %#v, want %#v", got, fixture.Initial)
	}
	for index, mark := range got {
		want := fixture.Initial[index]
		if uint64(mark.ID()) != want.ID || int(mark.Range().Start) != want.Start ||
			int(mark.Range().End) != want.End || mark.Level() != want.Level {
			t.Fatalf("mark %d = %#v, want %#v", index, mark, want)
		}
	}
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		if err := draft.SetMarkLevel(node, nested, 0); err != nil {
			return err
		}
		return draft.SetMarkLevel(node, first, 2)
	}); err != nil {
		t.Fatal(err)
	}
	value, _ = document.Snapshot().Node(node)
	got = value.Marks()
	if len(got) != len(fixture.Final) {
		t.Fatalf("final marks = %#v, want %#v", got, fixture.Final)
	}
	for index, mark := range got {
		want := fixture.Final[index]
		if uint64(mark.ID()) != want.ID || int(mark.Range().Start) != want.Start ||
			int(mark.Range().End) != want.End || mark.Level() != want.Level {
			t.Fatalf("final mark %d = %#v, want %#v", index, mark, want)
		}
	}
}
