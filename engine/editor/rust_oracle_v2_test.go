package editor

import (
	"encoding/json"
	"os"
	"testing"
	"unicode"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

type rustOracleV2Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type rustOracleV2WordCase struct {
	Text           string `json:"text"`
	UTF16Len       int    `json:"utf16_len"`
	Characters     int    `json:"characters"`
	Words          int    `json:"words"`
	Paragraphs     int    `json:"paragraphs"`
	WordEndOffsets []int  `json:"word_end_offsets"`
}

type rustOracleV2WordFixture struct {
	Fixture    string                 `json:"fixture"`
	RustCommit string                 `json:"rust_commit"`
	Cases      []rustOracleV2WordCase `json:"cases"`
}

type rustOracleV2State struct {
	Enabled  bool `json:"enabled"`
	Selected bool `json:"selected"`
}

type rustOracleV2StrongFixture struct {
	Fixture          string            `json:"fixture"`
	RustCommit       string            `json:"rust_commit"`
	AllowStrong      bool              `json:"allow_strong"`
	StateBefore      rustOracleV2State `json:"state_before"`
	StateAfter       rustOracleV2State `json:"state_after"`
	ProjectionBefore string            `json:"projection_before"`
	ProjectionAfter  string            `json:"projection_after"`
	SemanticText     string            `json:"semantic_text"`
	Result           struct {
		ErrorVariant string `json:"error_variant"`
		ErrorDisplay string `json:"error_display"`
	} `json:"result"`
}

type rustOracleV2AlphabeticFixture struct {
	Fixture        string `json:"fixture"`
	RustCommit     string `json:"rust_commit"`
	Codepoint      string `json:"codepoint"`
	IsAlphanumeric bool   `json:"is_alphanumeric"`
	Pair           struct {
		Prefix          string            `json:"prefix"`
		Typed           string            `json:"typed"`
		PairsEnabled    bool              `json:"pairs_enabled"`
		DiscoveredPairs int               `json:"discovered_pairs"`
		Projection      string            `json:"projection"`
		SemanticText    string            `json:"semantic_text"`
		Caret           rustOracleV2Range `json:"caret"`
	} `json:"pair"`
	Typography struct {
		Prefix       string            `json:"prefix"`
		Typed        string            `json:"typed"`
		PairsEnabled bool              `json:"pairs_enabled"`
		Projection   string            `json:"projection"`
		SemanticText string            `json:"semantic_text"`
		Caret        rustOracleV2Range `json:"caret"`
	} `json:"typography"`
}

type rustOracleV2CompoundIMEFixture struct {
	Fixture                 string            `json:"fixture"`
	RustCommit              string            `json:"rust_commit"`
	PreeditAfterInitial     string            `json:"preedit_after_initial"`
	PreeditAfterReplacement string            `json:"preedit_after_replacement"`
	Outcome                 string            `json:"outcome"`
	Projection              string            `json:"projection"`
	SemanticText            string            `json:"semantic_text"`
	Caret                   rustOracleV2Range `json:"caret"`
	CompositionCleared      bool              `json:"composition_cleared"`
}

type rustOracleV2UndoFixture struct {
	Fixture               string            `json:"fixture"`
	RustCommit            string            `json:"rust_commit"`
	Typing                []string          `json:"typing"`
	SelectionBeforeFormat rustOracleV2Range `json:"selection_before_format"`
	AfterFormat           struct {
		Projection      string            `json:"projection"`
		VisualSelection rustOracleV2Range `json:"visual_selection"`
	} `json:"after_format"`
	AfterUndo struct {
		Projection      string            `json:"projection"`
		VisualSelection rustOracleV2Range `json:"visual_selection"`
		SemanticText    string            `json:"semantic_text"`
		Caret           rustOracleV2Range `json:"caret"`
	} `json:"after_undo"`
	CanUndo bool `json:"can_undo"`
	CanRedo bool `json:"can_redo"`
}

func loadRustOracleV2(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("../../fixtures/rust-oracle/v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]json.RawMessage, len(entries))
	for _, entry := range entries {
		var name string
		if err := json.Unmarshal(entry["fixture"], &name); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = raw
	}
	return out
}

func TestRustOracleV2Manifest(t *testing.T) {
	t.Parallel()
	fixtures := loadRustOracleV2(t)
	want := []string{
		"word-like-v2",
		"compound-ime-strong-v2",
		"undo-marker-selection-v2",
		"allow-strong-v2",
		"other-alphabetic-v2",
		"kernel-document-authority-v2",
	}
	var commit string
	for _, name := range want {
		raw, ok := fixtures[name]
		if !ok {
			t.Fatalf("fixture %q missing", name)
		}
		var header struct {
			Fixture    string `json:"fixture"`
			RustCommit string `json:"rust_commit"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			t.Fatal(err)
		}
		if header.Fixture != name || len(header.RustCommit) != 40 {
			t.Fatalf("fixture header = %#v", header)
		}
		if commit == "" {
			commit = header.RustCommit
		} else if header.RustCommit != commit {
			t.Fatalf("mixed Rust commits %q and %q", commit, header.RustCommit)
		}
	}
	if len(fixtures) != len(want) {
		t.Fatalf("fixture count = %d, want %d", len(fixtures), len(want))
	}
}

func TestRustOracleV2WordEvidence(t *testing.T) {
	t.Parallel()
	var fixture rustOracleV2WordFixture
	if err := json.Unmarshal(loadRustOracleV2(t)["word-like-v2"], &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		name := test.Text
		t.Run(name, func(t *testing.T) {
			metrics, ok := ComputeTextMetrics(test.Text)
			if !ok {
				t.Fatal("current metrics rejected valid UTF-8")
			}
			if got := model.StringLenUTF16(test.Text); got != test.UTF16Len {
				t.Fatalf("UTF-16 length = %d, want %d", got, test.UTF16Len)
			}
			if metrics.Characters != test.Characters || metrics.Paragraphs != test.Paragraphs {
				t.Fatalf("shared metrics = %#v, want characters=%d paragraphs=%d",
					metrics, test.Characters, test.Paragraphs)
			}
			if metrics.Words != test.Words {
				t.Fatalf("word count: got %d, Rust oracle has %d",
					metrics.Words, test.Words)
			}
			ends := make([]int, 0, len(test.WordEndOffsets))
			head := model.UTF16Offset(0)
			for {
				plan, err := PlanWordCommand(
					test.Text,
					WordSelection{Anchor: head, Head: head},
					WordNext,
				)
				if err != nil {
					t.Fatalf("word movement at %d: %v", head, err)
				}
				if plan.Head == head {
					break
				}
				head = plan.Head
				ends = append(ends, int(head))
			}
			if len(ends) != len(test.WordEndOffsets) {
				t.Fatalf("word ends = %v, want %v", ends, test.WordEndOffsets)
			}
			for index := range ends {
				if ends[index] != test.WordEndOffsets[index] {
					t.Fatalf("word ends = %v, want %v", ends, test.WordEndOffsets)
				}
			}
		})
	}
}

func TestRustOracleV2OtherAlphabeticObservableFacts(t *testing.T) {
	t.Parallel()
	var fixture rustOracleV2AlphabeticFixture
	if err := json.Unmarshal(loadRustOracleV2(t)["other-alphabetic-v2"], &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Codepoint != "U+0345" {
		t.Fatalf("codepoint = %q", fixture.Codepoint)
	}
	mark := rune(0x0345)
	if !fixture.IsAlphanumeric || !unicode.Is(unicode.Other_Alphabetic, mark) {
		t.Fatal("U+0345 is not Other_Alphabetic")
	}
	if !ShouldGroupTyping(kernel.EventOriginTyping, true, string(mark)) {
		t.Fatal("Other_Alphabetic typing does not group")
	}

	t.Run("pair", func(t *testing.T) {
		session, _ := newSessionFixture(t, fixture.Pair.Prefix, false)
		outcome, err := session.ExecuteText(TextDocumentEnd)
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("document end outcome=%v err=%v", outcome, err)
		}
		result, err := session.ReplaceSelection(fixture.Pair.Typed, kernel.EventOriginTyping)
		if err != nil || result.Outcome != kernel.OutcomeHandled {
			t.Fatalf("pair typing outcome=%v err=%v", result.Outcome, err)
		}
		if session.Projection().Text() != fixture.Pair.Projection ||
			len(session.Pairs()) != fixture.Pair.DiscoveredPairs {
			t.Fatalf("pair state: projection=%q pairs=%d, oracle projection=%q pairs=%d",
				session.Projection().Text(), len(session.Pairs()),
				fixture.Pair.Projection, fixture.Pair.DiscoveredPairs)
		}
	})

	t.Run("typography", func(t *testing.T) {
		session, _ := newSessionFixture(t, fixture.Typography.Prefix, false)
		_, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
			configuration.SetEnabled(PairsPolicyID, false)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := session.ExecuteText(TextDocumentEnd)
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("document end outcome=%v err=%v", outcome, err)
		}
		result, err := session.ReplaceSelection(fixture.Typography.Typed, kernel.EventOriginTyping)
		if err != nil || result.Outcome != kernel.OutcomeHandled {
			t.Fatalf("typography typing outcome=%v err=%v", result.Outcome, err)
		}
		if session.Projection().Text() != fixture.Typography.Projection {
			t.Fatalf("typography projection=%q, oracle projection=%q",
				session.Projection().Text(), fixture.Typography.Projection)
		}
	})
}

func TestRustOracleV2AllowStrongObservableState(t *testing.T) {
	t.Parallel()
	var fixture rustOracleV2StrongFixture
	if err := json.Unmarshal(loadRustOracleV2(t)["allow-strong-v2"], &fixture); err != nil {
		t.Fatal(err)
	}
	editing := newEditingFixture(t, fixture.SemanticText)
	node := editing.nodeValue(t)
	settings := MarkupSettings{
		PolicyEnabled:  true,
		CommandEnabled: true,
		AllowStrong:    fixture.AllowStrong,
	}
	state := MarkupStateFor(
		MarkupStrong,
		settings,
		node,
		editing.selection(t, 0, model.UTF16Offset(model.StringLenUTF16(fixture.SemanticText))),
		nil,
	)
	if state.Enabled != fixture.StateBefore.Enabled ||
		state.Selected != fixture.StateBefore.Selected ||
		state.Enabled != fixture.StateAfter.Enabled ||
		state.Selected != fixture.StateAfter.Selected {
		t.Fatalf("state = %#v, oracle before=%#v after=%#v",
			state, fixture.StateBefore, fixture.StateAfter)
	}
	if fixture.ProjectionBefore != fixture.ProjectionAfter ||
		fixture.ProjectionBefore != fixture.SemanticText {
		t.Fatalf("strong rejection changed projection: %#v", fixture)
	}
}

func TestRustOracleV2BlockingMutations(t *testing.T) {
	t.Parallel()
	fixtures := loadRustOracleV2(t)

	t.Run("compound IME replaces final preedit inside strong", func(t *testing.T) {
		var fixture rustOracleV2CompoundIMEFixture
		if err := json.Unmarshal(fixtures["compound-ime-strong-v2"], &fixture); err != nil {
			t.Fatal(err)
		}
		session, node := newSessionFixture(t, "", false)
		outcome, err := session.ExecuteMarkup(MarkupStrong)
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("strong outcome=%v err=%v", outcome, err)
		}
		result, err := session.ReplaceSelection("a", kernel.EventOriginTyping)
		if err != nil || result.Outcome != kernel.OutcomeHandled {
			t.Fatalf("typing outcome=%v err=%v", result.Outcome, err)
		}
		if err := session.BeginComposition(); err != nil {
			t.Fatalf("begin composition: %v", err)
		}
		if err := session.UpdateComposition("\u306b"); err != nil {
			t.Fatalf("initial preedit: %v", err)
		}
		preview, _ := session.Composition().Preview()
		if preview.Text() != fixture.PreeditAfterInitial {
			t.Fatalf("initial preedit = %q", preview.Text())
		}
		if err := session.UpdateComposition("b"); err != nil {
			t.Fatalf("replacement preedit: %v", err)
		}
		preview, _ = session.Composition().Preview()
		if preview.Text() != fixture.PreeditAfterReplacement {
			t.Fatalf("replacement preedit = %q", preview.Text())
		}
		outcome, err = session.CommitComposition()
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("composition commit: outcome=%v err=%v, oracle projection=%q",
				outcome, err, fixture.Projection)
		}
		nodeValue, ok := session.Runtime().Snapshot().Document().Node(node)
		if !ok {
			t.Fatal("document node missing")
		}
		caret := session.Caret().RangeUTF16()
		_, compositionActive := session.Composition().Preview()
		if session.Projection().Text() != fixture.Projection ||
			nodeValue.Text().String() != fixture.SemanticText ||
			int(caret.Start) != fixture.Caret.Start || int(caret.End) != fixture.Caret.End ||
			compositionActive != !fixture.CompositionCleared {
			t.Fatalf("composition state: projection=%q semantic=%q caret=%#v active=%v",
				session.Projection().Text(), nodeValue.Text().String(), caret,
				compositionActive)
		}
	})

	t.Run("undo restores marker-only visual selection", func(t *testing.T) {
		var fixture rustOracleV2UndoFixture
		if err := json.Unmarshal(fixtures["undo-marker-selection-v2"], &fixture); err != nil {
			t.Fatal(err)
		}
		session, node := newSessionFixture(t, "", false)
		for _, text := range fixture.Typing {
			result, err := session.ReplaceSelection(text, kernel.EventOriginTyping)
			if err != nil || result.Outcome != kernel.OutcomeHandled {
				t.Fatalf("typing %q outcome=%v err=%v", text, result.Outcome, err)
			}
		}
		if err := session.SelectProjected(
			model.UTF16Offset(fixture.SelectionBeforeFormat.Start),
			model.UTF16Offset(fixture.SelectionBeforeFormat.End),
		); err != nil {
			t.Fatal(err)
		}
		outcome, err := session.ExecuteMarkup(MarkupStrong)
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("strong outcome=%v err=%v", outcome, err)
		}
		visual, err := session.VisualSelection()
		if err != nil ||
			session.Projection().Text() != fixture.AfterFormat.Projection ||
			int(visual.Start) != fixture.AfterFormat.VisualSelection.Start ||
			int(visual.End) != fixture.AfterFormat.VisualSelection.End {
			t.Fatalf("format state projection=%q visual=%#v err=%v",
				session.Projection().Text(), visual, err)
		}
		outcome, err = session.ExecuteHistory(HistoryUndo)
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("undo outcome=%v err=%v", outcome, err)
		}
		visual, err = session.VisualSelection()
		nodeValue, ok := session.Runtime().Snapshot().Document().Node(node)
		if !ok {
			t.Fatal("document node missing")
		}
		caret := session.Caret().RangeUTF16()
		if err != nil ||
			session.Projection().Text() != fixture.AfterUndo.Projection ||
			int(visual.Start) != fixture.AfterUndo.VisualSelection.Start ||
			int(visual.End) != fixture.AfterUndo.VisualSelection.End ||
			nodeValue.Text().String() != fixture.AfterUndo.SemanticText ||
			int(caret.Start) != fixture.AfterUndo.Caret.Start ||
			int(caret.End) != fixture.AfterUndo.Caret.End ||
			session.History().CanUndo() != fixture.CanUndo ||
			session.History().CanRedo() != fixture.CanRedo {
			t.Fatalf("undo state: projection=%q visual=%#v caret=%#v err=%v",
				session.Projection().Text(), visual, caret, err)
		}
	})

	t.Run("allow-strong false rejects strong formatting", func(t *testing.T) {
		var fixture rustOracleV2StrongFixture
		if err := json.Unmarshal(fixtures["allow-strong-v2"], &fixture); err != nil {
			t.Fatal(err)
		}
		session, _ := newSessionFixture(t, fixture.SemanticText, false)
		outcome, err := session.ExecuteText(TextSelectAll)
		if err != nil || outcome != kernel.OutcomeHandled {
			t.Fatalf("select all outcome=%v err=%v", outcome, err)
		}
		_, err = session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
			configuration.SetSetting(NewMarkupSettingKey("allow-strong"), kernel.BoolSetting(false))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		outcome, err = session.ExecuteMarkup(MarkupStrong)
		if err == nil {
			t.Fatalf("strong formatting returned %v without rejection", outcome)
		}
		if err.Error() != fixture.Result.ErrorDisplay ||
			session.Projection().Text() != fixture.ProjectionAfter {
			t.Fatalf("strong rejection: error=%q projection=%q",
				err.Error(), session.Projection().Text())
		}
	})
}
