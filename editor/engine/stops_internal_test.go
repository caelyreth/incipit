package engine

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// A transaction that has changed nothing reads its stops through the
// view's presenter, as its snapshot; one that has changed nodes, which it
// changes in place, through a presenter of its own.
func TestAnUnchangedTransactionReadsAsItsSnapshot(t *testing.T) {
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile, Enabled: []*policy.Policy{core.Policy}, Strict: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 3, doc.NewInterner())
	e := New(snap, prog, Options{})
	e.OpenView(1)
	if _, err := e.Do(1, vocab.Input{Intent: vocab.Type{Text: "ab"}, At: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	v := e.views[1]
	view := e.presenter(1)
	tx := e.snap.Begin(doc.Origin{})
	m := e.stopsFor(1)(tx, v).(*stopModel)
	if m.pr != view || m.r != e.snap {
		t.Fatal("an unchanged transaction did not read as its snapshot")
	}
	ref, _ := vocab.FirstTextSlot(tx.Root())
	if err := tx.Apply(doc.EditText{At: ref, From: 0, To: 0, Items: []doc.Item{doc.TextItem("x", nil)}}); err != nil {
		t.Fatal(err)
	}
	if m := e.stopsFor(1)(tx, v).(*stopModel); m.pr == view || m.r != doc.Reader(tx) {
		t.Fatal("a changed transaction read through the view's presenter")
	}
}
