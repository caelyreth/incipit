package policy

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func swallowAll[I vocab.Intent, T any](*Ctx, I, At[T]) Result { return Swallow }

// testCore answers every intent at the slot with a swallow, naming itself.
var testCore = Define(CoreID, func(p *P) {
	for _, k := range vocab.IntentKinds() {
		On(p, "fallback-"+k.String(), Kind(k), Slot.Inside(), swallowAll).Any()
	}
})

type paren struct{ open, close vocab.Span }

func (p paren) Sides() (vocab.Edge, vocab.Edge)                { return edge(p.open), edge(p.close) }
func (p paren) EditSide(vocab.Side, vocab.Spelling) vocab.Edit { return nil }
func (p paren) Exit() doc.Caret                                { return doc.CaretAt(doc.At(p.close.Slot, p.close.To)) }
func (p paren) Entry() doc.Caret                               { return doc.CaretAt(doc.At(p.close.Slot, p.close.From)) }

func edge(s vocab.Span) vocab.Edge {
	return vocab.Edge{From: doc.CaretAt(doc.At(s.Slot, s.From)), To: doc.CaretAt(doc.At(s.Slot, s.To))}
}

// parenScanner finds balanced parentheses; its state is the stack of
// opener offsets, packed in a string so it stays comparable.
type parenScanner struct{ kind *FactKind[paren] }

// slot: the test page has one paragraph, whose slot the token cannot name;
// the test fills it in through the package-level ref.
func (s parenScanner) slot(Token) doc.SlotRef { return testRef }

var testRef doc.SlotRef

func (s parenScanner) Start(*Unit) string { return "" }
func (s parenScanner) Step(st string, t Token, emit func(Fact)) string {
	parenSteps++
	if t.Kind != TokenText {
		return st
	}
	for i := 0; i < len(t.Text); i++ {
		switch t.Text[i] {
		case '(':
			st += string(rune(t.At + i + 1))
		case ')':
			if st == "" {
				continue
			}
			r := []rune(st)
			open := int(r[len(r)-1]) - 1
			st = string(r[:len(r)-1])
			at := t.At + i
			emit(s.kind.Delimited(open, open+1, at, at+1, paren{open: vocab.Span{Slot: s.slot(t), From: open, To: open + 1}, close: vocab.Span{Slot: s.slot(t), From: at, To: at + 1}}))
		}
	}
	return st
}

// parenSteps counts the tokens parenScanner steps over.
var parenSteps int

var testPairs = Define("pairs", func(p *P) {
	kind := Facts[paren](p, "paren", Provides(func(at At[paren]) paren { return at.Data }, vocab.RolePair, vocab.RoleLeavable))
	Scan[string](p, parenScanner{kind: kind}, []FactID{kind.Kind()})
	On(p, "leave", Intent[vocab.Leave](), Role[vocab.Leavable](vocab.RoleLeavable).Inside(), func(_ *Ctx, _ vocab.Leave, at At[vocab.Leavable]) Result {
		return Select(doc.Collapsed(at.Data.Exit()))
	})
	On(p, "overtype", Typed(")"), Or(kind.AtEnd(), kind.Empty()), func(_ *Ctx, _ vocab.Type, at At[paren]) Result {
		return Select(doc.Collapsed(doc.CaretAt(doc.At(at.Slot, at.Fact.Outer.To.At))))
	})
	On(p, "soft", Intent[vocab.SoftBreak](), kind.Inside(), func(*Ctx, vocab.SoftBreak, At[paren]) Result { return Pass })
})

// testParen declares the paren fact kind in a policy of its own.
func testParen(p *P) *FactKind[paren] {
	kind := Facts[paren](p, "paren", Provides(func(at At[paren]) paren { return at.Data }, vocab.RolePair))
	Scan[string](p, parenScanner{kind: kind}, []FactID{kind.Kind()})
	return kind
}

func testPage(t *testing.T, text string) (*doc.Snapshot, doc.SlotRef) {
	t.Helper()
	snap, err := doc.NewSnapshot(profile.Schema(), doc.NewInterner(), doc.NewNodeWithID(1, profile.Page, doc.Attrs{}, "", doc.BlockSlot()), 7)
	if err != nil {
		t.Fatal(err)
	}
	tx := snap.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Insert{Parent: 1, Nodes: []*doc.Node{profile.NewParagraph(doc.TextItem(text, nil))}}); err != nil {
		t.Fatal(err)
	}
	snap, err = tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	testRef = doc.SlotRef{Node: snap.Root().Slot(0).Child(0).ID()}
	return snap, testRef
}

func run(t *testing.T, prog *Program, store *Store, snap doc.Reader, at doc.Pos, in vocab.Intent) Outcome {
	t.Helper()
	view := &View{Selection: doc.Collapsed(doc.CaretAt(at))}
	out, err := prog.Dispatch(NewCtx(prog, store, snap, view, vocab.Input{Intent: in}, positions))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDispatchInnermostOutward(t *testing.T) {
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, testPairs}, Strict: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, ref := testPage(t, "a(b(c)d)e")
	store := NewStore()
	// Inside the inner pair, Leave exits the inner one.
	out := run(t, prog, store, snap, doc.At(ref, 5), vocab.Leave{})
	if out.Rule != "pairs.leave" || out.Result.Selection.Head.Pos.Offset != 6 {
		t.Fatalf("got %s %+v\n%s", out.Rule, out.Result.Selection, out.Trace)
	}
	// Between the pairs, Leave exits the outer one.
	out = run(t, prog, store, snap, doc.At(ref, 7), vocab.Leave{})
	if out.Result.Selection.Head.Pos.Offset != 8 {
		t.Fatalf("got %+v", out.Result.Selection)
	}
	// Outside every pair, core answers.
	out = run(t, prog, store, snap, doc.At(ref, 0), vocab.Leave{})
	if out.Policy != CoreID {
		t.Fatalf("got %s", out.Rule)
	}
	// Overtype needs the inner end: at offset 5 it holds, at 4 it doesn't.
	if out := run(t, prog, store, snap, doc.At(ref, 5), vocab.Type{Text: ")"}); out.Rule != "pairs.overtype" {
		t.Fatalf("got %s", out.Rule)
	}
	if out := run(t, prog, store, snap, doc.At(ref, 4), vocab.Type{Text: ")"}); out.Policy != CoreID {
		t.Fatalf("got %s", out.Rule)
	}
	// A rule that passes leaves the intent to the next one: here the same
	// rule in the outer pair, then core.
	out = run(t, prog, store, snap, doc.At(ref, 5), vocab.SoftBreak{})
	if len(out.Trace.Passed) != 2 || out.Rule != "core.fallback-SoftBreak" {
		t.Fatalf("got %s %v", out.Rule, out.Trace.Passed)
	}
}

func TestCompileChecks(t *testing.T) {
	noCore := Define(CoreID, func(p *P) {})
	_, err := Compile(Manifest{Enabled: []*Policy{noCore}}, nil)
	var ce *CompileError
	if !errors.As(err, &ce) || !strings.Contains(ce.Error(), "no fallback") {
		t.Fatalf("missing fallbacks not reported: %v", err)
	}
	a := Define("a", func(p *P) { On(p, "x", Typed("*"), Slot.Inside(), swallowAll) })
	b := Define("b", func(p *P) { On(p, "y", Intent[vocab.Type](), Slot.Inside(), swallowAll) })
	if _, err := Compile(Manifest{Enabled: []*Policy{testCore, a, b}}, nil); err == nil || !strings.Contains(err.Error(), "must order") {
		t.Fatalf("unordered overlap not reported: %v", err)
	}
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, a, b}, Resolve: []Order{Over(a, b, vocab.IntentType)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, ref := testPage(t, "x")
	if out := run(t, prog, NewStore(), snap, doc.At(ref, 0), vocab.Type{Text: "*"}); out.Rule != "a.x" {
		t.Fatalf("got %s", out.Rule)
	}
	// Places that cannot hold at once need no order: a pair's start and
	// its end are different zones.
	starts := Define("starts", func(p *P) {
		On(p, "x", Typed(")"), testParen(p).AtStart(), swallowAll)
	})
	ends := Define("ends", func(p *P) {
		On(p, "y", Typed(")"), Role[vocab.Pair](vocab.RolePair).AtEnd(), swallowAll)
	})
	if _, err := Compile(Manifest{Enabled: []*Policy{testCore, starts, ends}}, nil); err != nil {
		t.Fatalf("disjoint places reported: %v", err)
	}
	// A rule after one that answers wherever it would is reported.
	shadowed := Define("shadowed", func(p *P) {
		On(p, "all", Intent[vocab.Type](), Slot.Inside(), swallowAll)
		On(p, "star", Typed("*"), Slot.Own(), swallowAll)
	})
	prog, err = Compile(Manifest{Enabled: []*Policy{testCore, shadowed}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := prog.Report().Shadowed; !slices.Equal(got, []string{"shadowed.star by shadowed.all"}) {
		t.Fatalf("shadowed = %v", got)
	}
	cyc := []Order{Over(a, b, nil), Over(b, a, nil)}
	if _, err := Compile(Manifest{Enabled: []*Policy{testCore, a, b}, Resolve: cyc}, nil); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle not reported: %v", err)
	}
	// A disabled policy stays in the catalog but adds no rules.
	prog, err = Compile(Manifest{Enabled: []*Policy{testCore, a, b}}, Values{EnabledKey("b"): false})
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Catalog().Policies) != 3 || prog.Catalog().Policies[2].Enabled {
		t.Fatalf("catalog: %+v", prog.Catalog().Policies)
	}
}

// Every owner keeps its own re-check for a shared core annotation kind.
func TestAnnotationChecksStayWithTheirOwner(t *testing.T) {
	always := Define("always", func(p *P) {
		p.Annotation(Armed, func(doc.Reader, Annotation) bool { return true })
	})
	never := Define("never", func(p *P) {
		p.Annotation(Armed, func(doc.Reader, Annotation) bool { return false })
	})
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, always, never}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := testPage(t, "x")
	for _, tc := range []struct {
		owner string
		want  bool
	}{
		{"always", true},
		{"never", false},
		{"gone", true},
	} {
		got := prog.CheckAnnotation(snap, Annotation{Kind: Armed, Owner: tc.owner})
		if got != tc.want {
			t.Errorf("owner %s: check = %v, want %v", tc.owner, got, tc.want)
		}
	}
}

func TestADuplicateAnnotationDeclarationFailsCompile(t *testing.T) {
	dup := Define("dup", func(p *P) {
		p.Annotation(Armed, nil)
		p.Annotation(Armed, nil)
	})
	_, err := Compile(Manifest{Enabled: []*Policy{testCore, dup}}, nil)
	var ce *CompileError
	if !errors.As(err, &ce) || !strings.Contains(ce.Error(), "declares annotation armed twice") {
		t.Fatalf("duplicate declaration not reported: %v", err)
	}
}

// The incremental scan gives the facts a scan from scratch gives.
func TestScannerResumesAndReuses(t *testing.T) {
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, testPairs}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("(word) ", 200)
	snap, ref := testPage(t, long+"(x"+long)
	store := NewStore()
	kind := prog.scopeKinds[0]
	view := &View{}
	before := prog.facts(store, snap, view, ref, kind)
	tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
	if err := tx.Apply(doc.EditText{At: ref, From: len(long) + 2, To: len(long) + 2, Items: []doc.Item{doc.TextItem(")", nil)}}); err != nil {
		t.Fatal(err)
	}
	next, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	inc := prog.facts(store, next, view, ref, kind)
	fresh := prog.facts(NewStore(), next, view, ref, kind)
	if !slices.EqualFunc(inc, fresh, func(a, b Fact) bool { return a.Outer == b.Outer && a.Inner == b.Inner }) {
		t.Fatalf("incremental %d facts, fresh %d", len(inc), len(fresh))
	}
	if len(fresh) != len(before)+1 {
		t.Fatalf("expected one more pair: %d -> %d", len(before), len(fresh))
	}
}

// In one long line, a scan resumes and stops about the edit, and gives the
// facts a scan from scratch gives.
func TestALongLineIsScannedAboutTheEdit(t *testing.T) {
	prog, err := Compile(Manifest{Enabled: []*Policy{testCore, testPairs}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Under 0xD800 bytes: parenScanner keeps offsets as runes.
	text := strings.Repeat("some (words) and more words after them. ", 1200)
	snap, ref := testPage(t, text)
	store := NewStore()
	kind := prog.scopeKinds[0]
	view := &View{}
	prog.facts(store, snap, view, ref, kind)
	rng := rand.New(rand.NewPCG(1, 2))
	for step := range 40 {
		n, _ := snap.Node(ref.Node)
		in := n.Slot(ref.Slot).Content()
		at := rng.IntN(in.Len())
		edit := doc.EditText{At: ref, From: at, To: at, Items: []doc.Item{doc.TextItem([]string{"x", "(", ")", "yz "}[step%4], nil)}}
		if step%5 == 4 {
			edit = doc.EditText{At: ref, From: at, To: min(in.Len(), at+1+rng.IntN(30))}
		}
		tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
		if err := tx.Apply(edit); err != nil {
			t.Fatal(err)
		}
		if snap, err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		parenSteps = 0
		inc := prog.facts(store, snap, view, ref, kind)
		steps := parenSteps
		fresh := prog.facts(NewStore(), snap, view, ref, kind)
		if !slices.EqualFunc(inc, fresh, func(a, b Fact) bool { return a.Outer == b.Outer && a.Inner == b.Inner }) {
			t.Fatalf("step %d: incremental %d facts, fresh %d", step, len(inc), len(fresh))
		}
		if all := parenSteps - steps; step%4 == 0 && steps > all/5 {
			t.Fatalf("step %d: a letter typed stepped %d of %d tokens", step, steps, all)
		}
	}
}

// positions is a stop model of one stop per position, for programs that
// present nothing.
func positions(doc.Reader, *View) Stops { return positionStops{} }

type positionStops struct{}

func (positionStops) Step(c doc.Caret, _ vocab.Direction) (doc.Caret, bool) { return c, false }
func (positionStops) Edge(vocab.Direction) (doc.Caret, bool)                { return doc.Caret{}, false }
func (positionStops) Normalize(c doc.Caret) doc.Caret                       { return c }
func (positionStops) Around(doc.Pos) (int, int)                             { return 0, 0 }
func (positionStops) Bound(c doc.Caret) doc.Pos                             { return c.Pos }
