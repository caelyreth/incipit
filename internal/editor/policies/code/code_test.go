package code

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/policies/core"
	"github.com/caelyreth/incipit/internal/editor/policies/markdown"
	"github.com/caelyreth/incipit/internal/editor/policies/notation"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

// open is a page holding a code block in lang, its text written.
func open(t *testing.T, lang, text string) *page {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, notation.Policy, markdown.Policy, Policy}, Strict: true,
		Resolve: []policy.Order{policy.Over(notation.Policy, Policy, vocab.IntentBreak)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	p := &page{t: t, e: engine.New(snap, prog, engine.Options{}), now: time.Unix(10, 0)}
	p.e.OpenView(1)
	for _, r := range "```" + lang {
		p.do(vocab.Type{Text: string(r)})
	}
	p.do(vocab.Break{})
	p.do(vocab.Paste{Text: text})
	return p
}

func (p *page) do(in vocab.Intent) {
	p.t.Helper()
	p.now = p.now.Add(time.Second)
	if _, err := p.e.Do(1, vocab.Input{Intent: in, At: p.now}); err != nil {
		p.t.Fatal(err)
	}
}

// classes reads the block as its runs: each run's text and the code
// group styling it, "word:keyword". A run ends at a line break, so a
// string over two lines is two runs.
func (p *page) classes() []string {
	p.t.Helper()
	block := p.e.Snapshot().Root().Slot(0).Child(0)
	pres := p.e.Present(1, []doc.ID{block.ID()})[0]
	names := map[string]string{
		profile.GroupCodeKeyword.String(): "keyword", profile.GroupCodeType.String(): "type",
		profile.GroupCodeConstant.String(): "constant", profile.GroupCodeNumber.String(): "number",
		profile.GroupCodeString.String(): "string", profile.GroupCodeComment.String(): "comment",
	}
	var out []string
	for _, it := range pres.Surfaces[0].Flow.Items {
		if it.Kind != present.TextRun {
			continue
		}
		for _, g := range it.Groups {
			if n, ok := names[g.String()]; ok {
				out = append(out, it.Text+":"+n)
			}
		}
	}
	return out
}

func TestCodeIsColouredByItsLanguage(t *testing.T) {
	for _, tc := range []struct {
		lang, text string
		want       []string
	}{
		{"go", "func f() int { return 42 } // done", []string{"func:keyword", "int:type", "return:keyword", "42:number", "// done:comment"}},
		{"go", "s := `a\nb` + \"x\\\"y\"", []string{"`a:string", "b`:string", "\"x\\\"y\":string"}},
		{"python", "def f():\n    \"\"\"Doc\n    more\"\"\"\n    return None # no", []string{"def:keyword", "\"\"\"Doc:string", "    more\"\"\":string", "return:keyword", "None:constant", "# no:comment"}},
		{"sql", "SELECT id FROM t WHERE x = 'a' -- why", []string{"SELECT:keyword", "FROM:keyword", "WHERE:keyword", "'a':string", "-- why:comment"}},
		{"js", "/* a */ const x = null", []string{"/* a */:comment", "const:keyword", "null:constant"}},
		// A string its line ends is closed there.
		{"go", "\"open\nfunc", []string{"\"open:string", "func:keyword"}},
		// An unknown language is left as it is written.
		{"brainfuck", "func 42", nil},
		{"", "func 42", nil},
	} {
		p := open(t, tc.lang, tc.text)
		if got := p.classes(); fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s %q: got %q, want %q", tc.lang, tc.text, got, tc.want)
		}
	}
}

// TestColouringIsTheSameHoweverTheTextCameToBe edits a long block about
// its middle, so the scan resumes from checkpoints with the text cut
// where the last scan cut it, and reads it as a fresh scan does.
func TestColouringIsTheSameHoweverTheTextCameToBe(t *testing.T) {
	line := "x := \"a string\" /* c */ + 12 // tail\n"
	text := strings.Repeat(line, 40)
	edited := open(t, "go", text)
	mid := len(text) / 2
	edited.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: edited.e.Snapshot().Root().Slot(0).Child(0).ID()}, mid))})
	for _, r := range "/* opened" {
		edited.do(vocab.Type{Text: string(r)})
	}
	for range len("/* opened") {
		edited.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	}
	fresh := open(t, "go", text)
	if got, want := edited.classes(), fresh.classes(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("edited and back:\n%q\nfresh:\n%q", got, want)
	}
}
