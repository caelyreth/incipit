package editor

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

type editingFixture struct {
	document *model.Document
	node     model.NodeID
}

func newEditingFixture(t *testing.T, text string, marks ...model.UTF16Range) editingFixture {
	t.Helper()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, text); err != nil {
			return err
		}
		for _, mark := range marks {
			if _, err := draft.AddMark(node, mark, 1); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return editingFixture{document: document, node: node}
}

func (f editingFixture) nodeValue(t *testing.T) *model.Node {
	t.Helper()
	node, ok := f.document.Snapshot().Node(f.node)
	if !ok {
		t.Fatal("fixture node missing")
	}
	return node
}

func (f editingFixture) selection(t *testing.T, anchor, head model.UTF16Offset) model.TextSelection {
	t.Helper()
	revision := f.document.Snapshot().Revision()
	return model.NewTextSelection(
		model.NewTextAnchor(f.node, anchor, model.AffinityRight, revision),
		model.NewTextAnchor(f.node, head, model.AffinityRight, revision),
	)
}

func TestTabOfferChoosesNarrowestPairBeforeOuterPair(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "(a[b])")
	pairs, _ := DiscoverPairs("(a[b])")
	selection := fixture.selection(t, 3, 3)
	offer, ok, err := (Navigation{}).TabOffer(
		time.Now(),
		fixture.nodeValue(t),
		selection,
		pairs,
		nil,
		DefaultNavigationSettings(),
	)
	if err != nil || !ok || offer.Kind != TabPair || offer.Target.OffsetUTF16() != 5 || offer.Scope != model.NewUTF16Range(2, 5) {
		t.Fatalf("offer=%#v ok=%v err=%v", offer, ok, err)
	}
}

func TestTabOfferPrefersEmphasisAtPairScopeInteriorAndCarriesMark(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "(abc)", model.NewUTF16Range(1, 4))
	pairs, _ := DiscoverPairs("(abc)")
	markID := fixture.nodeValue(t).Marks()[0].ID()
	selection := fixture.selection(t, 2, 2)
	offer, ok, err := (Navigation{}).TabOffer(
		time.Now(),
		fixture.nodeValue(t),
		selection,
		pairs,
		&markID,
		DefaultNavigationSettings(),
	)
	if err != nil || !ok || offer.Kind != TabEmphasis || !offer.HasMark || offer.Mark != markID || offer.Target.OffsetUTF16() != 4 {
		t.Fatalf("offer=%#v ok=%v err=%v", offer, ok, err)
	}
}

func TestNavigationReturnRequiresEightGraphemesAndExpires(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "a\u0301👩‍👩‍👧‍👦🇨🇳z12345678")
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	start := fixture.selection(t, 0, 0)
	var nav Navigation
	nav.Remember(start)
	pointer := fixture.selection(t, 26, 26)
	if err := nav.SelectPointer(fixture.nodeValue(t).Text().String(), pointer, now); err != nil {
		t.Fatal(err)
	}
	target, ok := nav.Returning()
	if !ok || target.ExpiresAt != now.Add(14*time.Second) {
		t.Fatalf("returning=%#v ok=%v", target, ok)
	}
	offer, ok, err := nav.TabOffer(now, fixture.nodeValue(t), pointer, nil, nil, DefaultNavigationSettings())
	if err != nil || !ok || offer.Kind != TabReturnToEdit || !offer.HasExpiry {
		t.Fatalf("offer=%#v ok=%v err=%v", offer, ok, err)
	}
	if _, ok, _ := nav.TabOffer(now.Add(14*time.Second), fixture.nodeValue(t), pointer, nil, nil, DefaultNavigationSettings()); ok {
		t.Fatal("expired return offer survived")
	}

	near := fixture.selection(t, 2, 2)
	nav.Remember(fixture.selection(t, 0, 0))
	if err := nav.SelectPointer(fixture.nodeValue(t).Text().String(), near, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := nav.Returning(); ok {
		t.Fatal("short pointer move offered return")
	}
}

func TestTabOfferRequiresCollapsedSelectionAndHonorsGates(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "(abcdef)")
	pairs, _ := DiscoverPairs("(abcdef)")
	selection := fixture.selection(t, 2, 4)
	settings := DefaultNavigationSettings()
	if _, ok, _ := (Navigation{}).TabOffer(time.Now(), fixture.nodeValue(t), selection, pairs, nil, settings); ok {
		t.Fatal("noncollapsed selection produced scoped offer")
	}
	settings.PairsPolicyEnabled = false
	collapsed := fixture.selection(t, 3, 3)
	if _, ok, _ := (Navigation{}).TabOffer(time.Now(), fixture.nodeValue(t), collapsed, pairs, nil, settings); ok {
		t.Fatal("disabled pair policy produced offer")
	}
	if _, ok, _ := (Navigation{}).TabOffer(time.Now(), fixture.nodeValue(t), collapsed, pairs, nil, NavigationSettings{}); ok {
		t.Fatal("disabled navigation produced offer")
	}
}
