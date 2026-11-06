package editor

import (
	"testing"
)

// An edit prepares again only the blocks it makes: the others keep the
// flows they had, and the page joined from them is the page prepared
// afresh, numbered lists renumbered, styles, objects and breaks alike.
func TestAnEditPreparesOnlyTheBlocksItMakes(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Title\n1. one\n2. two\n- [ ] task $x$\nplain")
	before := session.pieces.kept
	if len(before) != 5 {
		t.Fatalf("the page keeps %d block flows", len(before))
	}
	// A new item at the list's head numbers the ones after it again.
	if err := session.SelectProjected(0, 0); err != nil {
		t.Fatal(err)
	}
	typeKeys(t, session, "A ")
	after := session.pieces.kept
	same := 0
	for key, flow := range after {
		if before[key] == flow {
			same++
		}
	}
	if same != 4 {
		t.Fatalf("%d of %d block flows were kept across an edit of one", same, len(after))
	}
	flow, err := session.SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := (*flowPieces)(nil).pageFlow(session.Projection(), session.Runtime().Snapshot().Document())
	if err != nil {
		t.Fatal(err)
	}
	if !flow.Equal(fresh) || flow.Text() != session.Projection().Text() {
		t.Fatalf("the joined page differs from one prepared afresh:\n%q\n%q", flow.Text(), fresh.Text())
	}
}
