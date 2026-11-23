package history

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

func TestTypingGroups(t *testing.T) {
	snap, err := profile.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := vocab.FirstTextSlot(snap.Root())
	text := "abcdef"
	entry := func(at int, after time.Duration) *doc.Entry {
		tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
		if err := tx.Apply(doc.EditText{At: ref, Items: []doc.Item{doc.TextItem(text[:at], nil)}}); err != nil {
			t.Fatal(err)
		}
		mark := len(tx.Steps())
		if err := tx.Apply(doc.EditText{At: ref, From: at, To: at, Items: []doc.Item{doc.TextItem("x", nil)}}); err != nil {
			t.Fatal(err)
		}
		return doc.NewEntry(tx.Steps()[mark:], doc.OriginTyping, 1, doc.Selection{}, doc.Selection{}, time.Unix(0, 0).Add(after))
	}
	window := 750 * time.Millisecond
	if !typingGroup(entry(0, 0), entry(1, 300*time.Millisecond), window) {
		t.Fatal("contiguous typing did not group")
	}
	if typingGroup(entry(0, 0), entry(1, time.Second), window) {
		t.Fatal("typing past the window grouped")
	}
	if typingGroup(entry(0, 0), entry(5, 0), window) {
		t.Fatal("typing elsewhere grouped")
	}
}
