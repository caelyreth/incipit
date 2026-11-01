package model

import (
	"slices"
	"testing"
)

func recordedDocument(t *testing.T) (*Document, NodeID) {
	t.Helper()
	document, err := NewDocument("doc")
	if err != nil {
		t.Fatal(err)
	}
	var block NodeID
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		block, err = draft.CreateNode(draft.Root(), 0, "paragraph")
		if err != nil {
			return err
		}
		if err := draft.SetTextValue(block, "hello world"); err != nil {
			return err
		}
		if err := draft.SetAttribute(block, "level", IntegerValue(2)); err != nil {
			return err
		}
		_, err = draft.AddMark(block, NewUTF16Range(0, 5), "strong", 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return document, block
}

func TestRecordsRestoreTheirDocument(t *testing.T) {
	document, _ := recordedDocument(t)
	snapshot := document.Snapshot()
	var records []NodeRecord
	for _, node := range snapshot.Nodes() {
		records = append(records, node.Record())
	}
	restored, err := RestoreDocument(snapshot.Root(), snapshot.NextID(), records)
	if err != nil {
		t.Fatal(err)
	}
	again := restored.Snapshot()
	if again.NextID() != snapshot.NextID() || len(again.Nodes()) != len(snapshot.Nodes()) {
		t.Fatalf("restored %d nodes, next %d", len(again.Nodes()), again.NextID())
	}
	for _, node := range snapshot.Nodes() {
		other, ok := again.Node(node.ID())
		if !ok || !sameNode(node, other) {
			t.Fatalf("node %s did not round-trip", node.ID())
		}
	}
}

func TestRestoreRejectsInvalidStoredText(t *testing.T) {
	document, block := recordedDocument(t)
	snapshot := document.Snapshot()
	records := make([]NodeRecord, 0, len(snapshot.Nodes()))
	for _, node := range snapshot.Nodes() {
		records = append(records, node.Record())
	}
	for _, test := range []struct {
		name string
		text string
	}{
		{name: "invalid UTF-8", text: string([]byte{0xff})},
		{name: "NUL", text: "hello\x00world"},
	} {
		t.Run(test.name, func(t *testing.T) {
			broken := append([]NodeRecord(nil), records...)
			text := test.text
			for index := range broken {
				if broken[index].ID == block {
					broken[index].Text = &text
					break
				}
			}
			if _, err := RestoreDocument(snapshot.Root(), snapshot.NextID(), broken); err == nil {
				t.Fatalf("restored %q", test.text)
			}
		})
	}
}

func TestRestorePreservesLegacyLineEndingsAndMarkOffsets(t *testing.T) {
	for _, text := range []string{"hello\r\nworld", "hello\rworld"} {
		document, block := recordedDocument(t)
		snapshot := document.Snapshot()
		var records []NodeRecord
		for _, node := range snapshot.Nodes() {
			record := node.Record()
			if record.ID == block {
				record.Text = &text
				record.Marks = []InlineMark{NewInlineMark(1, NewUTF16Range(0, 5), "strong", 1)}
			}
			records = append(records, record)
		}
		restored, err := RestoreDocument(snapshot.Root(), snapshot.NextID(), records)
		if err != nil {
			t.Fatalf("restore %q: %v", text, err)
		}
		node, _ := restored.Snapshot().Node(block)
		want := []InlineMark{NewInlineMark(1, NewUTF16Range(0, 5), "strong", 1)}
		if node.Text().String() != text || !slices.Equal(node.Marks(), want) {
			t.Fatalf("restore changed legacy text or offsets: %q %+v", node.Text().String(), node.Marks())
		}
		if _, err := restored.Transact(func(draft *DocumentDraft) error {
			return draft.SetAttribute(block, "note", TextValue("editable"))
		}); err != nil {
			t.Fatalf("edit legacy document %q: %v", text, err)
		}
	}
}

func TestRestorePreservesCanonicalUnicodeMarksAndOffsets(t *testing.T) {
	document, block := recordedDocument(t)
	snapshot := document.Snapshot()
	records := make([]NodeRecord, 0, len(snapshot.Nodes()))
	for _, node := range snapshot.Nodes() {
		records = append(records, node.Record())
	}
	const text = "a🦀\nb"
	for index := range records {
		if records[index].ID == block {
			value := text
			records[index].Text = &value
		}
	}
	restored, err := RestoreDocument(snapshot.Root(), snapshot.NextID(), records)
	if err != nil {
		t.Fatal(err)
	}
	node, ok := restored.Snapshot().Node(block)
	if !ok {
		t.Fatal("restored block is missing")
	}
	if got := node.Text().String(); got != text {
		t.Fatalf("text = %q", got)
	}
	marks := node.Marks()
	if len(marks) != 1 || marks[0].Range() != NewUTF16Range(0, 5) {
		t.Fatalf("marks = %#v", marks)
	}
	if err := node.ValidateTextRangeUTF16(0, UTF16Offset(node.Text().UTF16Len())); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRefusesABrokenTree(t *testing.T) {
	document, block := recordedDocument(t)
	snapshot := document.Snapshot()
	records := []NodeRecord{}
	for _, node := range snapshot.Nodes() {
		records = append(records, node.Record())
	}
	orphan := records[1]
	orphan.ID = 9
	cases := map[string][]NodeRecord{
		"orphan":     append(append([]NodeRecord{}, records...), orphan),
		"no child":   {records[0]},
		"duplicated": append(append([]NodeRecord{}, records...), records[1]),
	}
	for name, broken := range cases {
		if _, err := RestoreDocument(snapshot.Root(), 10, broken); err == nil {
			t.Errorf("%s: restored a broken tree", name)
		}
	}
	if _, err := RestoreDocument(snapshot.Root(), uint64(block), records); err == nil {
		t.Error("restored identities at or past the next identity")
	}
}

func TestSinceNamesOnlyWrittenNodes(t *testing.T) {
	document, block := recordedDocument(t)
	before := document.Snapshot()
	var added NodeID
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(block, "changed"); err != nil {
			return err
		}
		var err error
		added, err = draft.CreateNode(draft.Root(), 1, "paragraph")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	written, removed := document.Snapshot().Since(before)
	ids := map[NodeID]bool{}
	for _, node := range written {
		ids[node.ID()] = true
	}
	if len(written) != 3 || !ids[block] || !ids[added] || !ids[before.Root()] || len(removed) != 0 {
		t.Fatalf("written %v, removed %v", ids, removed)
	}
	middle := document.Snapshot()
	if _, err := document.Transact(func(draft *DocumentDraft) error {
		return draft.RemoveNode(added)
	}); err != nil {
		t.Fatal(err)
	}
	written, removed = document.Snapshot().Since(middle)
	if len(removed) != 1 || removed[0] != added || len(written) != 1 {
		t.Fatalf("written %d, removed %v", len(written), removed)
	}
	if written, removed := middle.Since(middle); written != nil || removed != nil {
		t.Fatal("a snapshot differs from itself")
	}
}
