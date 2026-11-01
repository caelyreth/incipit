package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func TestMarkupProjectNestedMarksWithoutStoringNotation(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "a🦀b")
	if _, err := fixture.document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(fixture.node, model.NewUTF16Range(0, 4), EmphasisMark, 1); err != nil {
			return err
		}
		_, err := draft.AddMark(fixture.node, model.NewUTF16Range(1, 3), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	configuration, err := markupConfiguration(t, true)
	if err != nil {
		t.Fatal(err)
	}
	projectionResult, err := ProjectPage(
		fixture.document.Snapshot(),
		fixture.document.Revision(),
		3,
		configuration,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := projectionResult.Text(), "*a**🦀**b*"; got != want {
		t.Fatalf("projected text=%q want %q", got, want)
	}
	if projectionResult.Version().Generation() != 3 ||
		projectionResult.Version().SourceRevision() != fixture.document.Revision() {
		t.Fatalf("version=%#v", projectionResult.Version())
	}
	if got := fixture.nodeValue(t).Text().String(); got != "a🦀b" {
		t.Fatalf("semantic text=%q", got)
	}
	segments := projectionResult.Segments()
	if len(segments) != 7 {
		t.Fatalf("segments=%#v", segments)
	}
	assertAttributeSegment(t, segments[0], MarkerKeyForMark(1), 0, 1)
	assertEditableSegment(t, segments[1], 1, 2)
	assertAttributeSegment(t, segments[2], MarkerKeyForMark(2), 2, 4)
	assertEditableSegment(t, segments[3], 4, 6)
	assertAttributeSegment(t, segments[4], MarkerKeyForMark(2), 6, 8)
	assertEditableSegment(t, segments[5], 8, 9)
	assertAttributeSegment(t, segments[6], MarkerKeyForMark(1), 9, 10)

	runs, err := InlineStyleRuns(projectionResult, fixture.document.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.Bytes.Start == 0 || run.Bytes.End == model.ByteOffset(len("*a**🦀**b*")) {
			if run.Style.Color != surface.ColorMuted || run.Style.Bold || run.Style.Italic {
				t.Fatalf("generated run=%#v", run)
			}
		}
	}
	if len(runs) != 7 ||
		runs[1].Style != (surface.TextStyle{Italic: true, Color: surface.ColorText}) ||
		runs[3].Style != (surface.TextStyle{Bold: true, Italic: true, Color: surface.ColorText}) ||
		runs[5].Style != (surface.TextStyle{Italic: true, Color: surface.ColorText}) {
		t.Fatalf("style runs=%#v", runs)
	}
}

func TestMarkupProjectDisjointAndZeroLengthMarksAdjoinMarkers(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abcd")
	if _, err := fixture.document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(fixture.node, model.EmptyUTF16Range(1), EmphasisMark, 1); err != nil {
			return err
		}
		_, err := draft.AddMark(fixture.node, model.NewUTF16Range(2, 3), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	configuration, err := markupConfiguration(t, true)
	if err != nil {
		t.Fatal(err)
	}
	projectionResult, err := ProjectPage(
		fixture.document.Snapshot(),
		fixture.document.Revision(),
		0,
		configuration,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := projectionResult.Text(), "a**b**c**d"; got != want {
		t.Fatalf("projected text=%q want %q", got, want)
	}
	var zeroBody projection.Segment
	foundZeroBody := false
	for _, segment := range projectionResult.Segments() {
		if target, ok := segment.SemanticTarget(); ok &&
			segment.Kind() == projection.SegmentEditable &&
			target.RangeUTF16() == model.EmptyUTF16Range(1) {
			zeroBody = segment
			foundZeroBody = true
		}
	}
	if !foundZeroBody || zeroBody.ProjectedRange() != model.EmptyUTF16Range(2) {
		t.Fatalf("zero-length body=%#v found=%v", zeroBody, foundZeroBody)
	}
}

func TestMarkupProjectMapsGeneratedMarkerExteriorsByAffinity(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "a🦀b")
	if _, err := fixture.document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(fixture.node, model.NewUTF16Range(0, 4), EmphasisMark, 1); err != nil {
			return err
		}
		_, err := draft.AddMark(fixture.node, model.NewUTF16Range(1, 3), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	configuration, err := markupConfiguration(t, true)
	if err != nil {
		t.Fatal(err)
	}
	projectionResult, err := ProjectPage(
		fixture.document.Snapshot(),
		fixture.document.Revision(),
		0,
		configuration,
	)
	if err != nil {
		t.Fatal(err)
	}
	revision := fixture.document.Revision()
	for _, test := range []struct {
		name     string
		offset   model.UTF16Offset
		affinity model.Affinity
		want     model.UTF16Offset
	}{
		{name: "outer start exterior", offset: 0, affinity: model.AffinityLeft, want: 0},
		{name: "outer start interior", offset: 0, affinity: model.AffinityRight, want: 1},
		{name: "inner start exterior", offset: 1, affinity: model.AffinityLeft, want: 2},
		{name: "inner start interior", offset: 1, affinity: model.AffinityRight, want: 4},
		{name: "inner end interior", offset: 3, affinity: model.AffinityLeft, want: 6},
		{name: "inner end exterior", offset: 3, affinity: model.AffinityRight, want: 8},
		{name: "outer end interior", offset: 4, affinity: model.AffinityLeft, want: 9},
		{name: "outer end exterior", offset: 4, affinity: model.AffinityRight, want: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			anchor := model.NewTextAnchor(fixture.node, test.offset, test.affinity, revision)
			target, err := projectionResult.ProjectAnchor(anchor)
			if err != nil {
				t.Fatal(err)
			}
			if target.RangeUTF16() != model.EmptyUTF16Range(test.want) {
				t.Fatalf(
					"target=%#v want offset %d (text=%q)",
					target.RangeUTF16(),
					test.want,
					projectionResult.Text(),
				)
			}
		})
	}
}

func TestMarkupProjectEmphasisAndSettings(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "ab")
	if _, err := fixture.document.Transact(func(draft *model.DocumentDraft) error {
		_, err := draft.AddMark(fixture.node, model.NewUTF16Range(0, 2), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Marks are built in: the writing policy being off changes what typing
	// makes, not what the page shows.
	for _, test := range []struct {
		name    string
		enabled bool
	}{
		{name: "writing on", enabled: true},
		{name: "writing off", enabled: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			configuration, err := markupConfiguration(t, test.enabled)
			if err != nil {
				t.Fatal(err)
			}
			projectionResult, err := ProjectPage(
				fixture.document.Snapshot(),
				fixture.document.Revision(),
				0,
				configuration,
			)
			if err != nil {
				t.Fatal(err)
			}
			if got := projectionResult.Text(); got != "**ab**" {
				t.Fatalf("text=%q", got)
			}
		})
	}
}

func TestEditorPolicyCatalogMatchesDeclarations(t *testing.T) {
	t.Parallel()
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	policies := catalog.Policies()
	if len(policies) != len(AllEditorPolicies)+1 {
		t.Fatalf("policies=%d want %d", len(policies), len(AllEditorPolicies)+1)
	}
	wantRequirements := map[kernel.PolicyID][]kernel.PolicyID{
		TextPolicyID:       nil,
		WritingPolicyID:    {TextPolicyID},
		PairsPolicyID:      {TextPolicyID},
		CJKPolicyID:        {TextPolicyID},
		TypographyPolicyID: {TextPolicyID},
		HistoryPolicyID:    {TextPolicyID},
		NavigationPolicyID: {TextPolicyID},
		WordsPolicyID:      {TextPolicyID},
		SurfacePolicyID:    {TextPolicyID},
	}
	for _, policy := range policies {
		if !slices.Equal(policy.Requires, wantRequirements[policy.ID]) {
			t.Fatalf("%s requires=%#v want %#v", policy.ID, policy.Requires, wantRequirements[policy.ID])
		}
		if policy.ID == SurfacePolicyID {
			if _, ok := EditorPolicyFromID(policy.ID); ok {
				t.Fatalf("surface host policy incorrectly resolved as an EditorPolicy")
			}
		} else if resolved, ok := EditorPolicyFromID(policy.ID); !ok || resolved.ID() != policy.ID {
			t.Fatalf("policy identity %s did not resolve", policy.ID)
		}
	}
	rules := map[kernel.PolicyID]map[kernel.RuleID]kernel.RuleDescriptor{}
	for _, entry := range catalog.Rules() {
		if rules[entry.Policy] == nil {
			rules[entry.Policy] = make(map[kernel.RuleID]kernel.RuleDescriptor)
		}
		rules[entry.Policy][entry.Rule.ID] = entry.Rule
	}
	assertRuleOrder(t, rules[TextPolicyID]["edit"], 100, kernel.RulePhaseKindHandle, kernel.TextInputKey)
	assertRuleOrder(t, rules[TextPolicyID]["projected-edit"], 100, kernel.RulePhaseKindHandle, kernel.ProjectedTextKey)
	assertRuleOrder(t, rules[WritingPolicyID]["type-marker"], 10, kernel.RulePhaseKindHandle, kernel.TextInputKey)
	if !slices.Equal(
		rules[WritingPolicyID]["type-marker"].Origins,
		[]kernel.EventOrigin{kernel.EventOriginTyping},
	) {
		t.Fatalf("marker origins=%#v", rules[WritingPolicyID]["type-marker"].Origins)
	}
	// Notation of the built-in kinds is core editing; the writing policy
	// only reads typed markers as structure.
	assertRuleOrder(t, rules[TextPolicyID]["notation"], 100, kernel.RulePhaseKindHandle, kernel.AttributeInputKey)
	assertRuleOrder(t, rules[TextPolicyID]["block"], 5, kernel.RulePhaseKindHandle, kernel.TextInputKey)
	assertRuleOrder(t, rules[WritingPolicyID]["block-syntax"], 6, kernel.RulePhaseKindHandle, kernel.TextInputKey)
	assertRuleOrder(t, rules[WritingPolicyID]["format"], 100, kernel.RulePhaseKindHandle, markupFormatTrigger)
	for id, order := range map[kernel.RuleID]uint32{
		"overtype": 14, "mate-edit": 16, "close-quote": 18, "open": 20,
	} {
		assertRuleOrder(t, rules[PairsPolicyID][id], order, kernel.RulePhaseKindHandle, kernel.TextInputKey)
	}
	if !slices.Equal(
		rules[PairsPolicyID]["mate-edit"].Origins,
		[]kernel.EventOrigin{kernel.EventOriginTyping, kernel.EventOriginSemantic},
	) {
		t.Fatalf("mate origins=%#v", rules[PairsPolicyID]["mate-edit"].Origins)
	}
	assertRuleOrder(t, rules[CJKPolicyID]["refine"], 300, kernel.RulePhaseKindAfter, kernel.TextInputKey)
	assertRuleOrder(t, rules[TypographyPolicyID]["refine"], 320, kernel.RulePhaseKindAfter, kernel.TextInputKey)
	if !slices.Equal(
		rules[CJKPolicyID]["refine"].Origins,
		[]kernel.EventOrigin{
			kernel.EventOriginTyping,
			kernel.EventOriginPaste,
			kernel.EventOriginIME,
		},
	) || !slices.Equal(
		rules[TypographyPolicyID]["refine"].Origins,
		[]kernel.EventOrigin{kernel.EventOriginTyping},
	) {
		t.Fatalf(
			"observer origins cjk=%#v typography=%#v",
			rules[CJKPolicyID]["refine"].Origins,
			rules[TypographyPolicyID]["refine"].Origins,
		)
	}
	if len(rules[HistoryPolicyID]) != 0 || len(rules[NavigationPolicyID]) != 0 ||
		len(rules[WordsPolicyID]) != 0 {
		t.Fatal("command-only policy declared a rule")
	}

	text := catalogPolicy(t, catalog, TextPolicyID)
	if got := len(text.Commands); got != len(AllTextCommands) {
		t.Fatalf("text commands=%d", got)
	}
	markup := catalogPolicy(t, catalog, WritingPolicyID)
	if !slices.Equal(markup.Commands, []kernel.CommandID{
		"markup.emphasis", "markup.strong", "writing.nest-deeper", "writing.nest-shallower",
		"table.insert", "table.row-above", "table.row-below", "table.column-left", "table.column-right",
		"table.delete-row", "table.delete-column", "table.delete", "table.header",
		"table.previous-cell", "table.line-break",
	}) ||
		!slices.Equal(settingIDs(markup.Settings), []kernel.SettingID{
			"asterisks", "underscores", "headings", "lists",
			"blockquotes", "code", "strikethrough", "rules", "tables",
		}) {
		t.Fatalf("markup=%#v", markup)
	}
	pairs := catalogPolicy(t, catalog, PairsPolicyID)
	if got := settingIDs(pairs.Settings); !slices.Equal(
		got,
		[]kernel.SettingID{"quotes", "overtype", "edit-mates", "contextual-quotes"},
	) {
		t.Fatalf("pair settings=%#v", got)
	}
	pairRuleOrder := make([]kernel.RuleID, 0, len(pairs.Rules))
	for _, rule := range pairs.Rules {
		pairRuleOrder = append(pairRuleOrder, rule.ID)
	}
	if !slices.Equal(
		pairRuleOrder,
		[]kernel.RuleID{"open", "overtype", "mate-edit", "mate-span", "close-quote"},
	) {
		t.Fatalf("pair declaration order=%#v", pairRuleOrder)
	}
	history := catalogPolicy(t, catalog, HistoryPolicyID)
	if got := commandIDs(history); !slices.Equal(got, []kernel.CommandID{"history.undo", "history.redo"}) {
		t.Fatalf("history commands=%#v", got)
	}
	navigation := catalogPolicy(t, catalog, NavigationPolicyID)
	if got := settingIDs(navigation.Settings); !slices.Equal(
		got,
		[]kernel.SettingID{"pairs", "emphasis", "return-to-edit"},
	) {
		t.Fatalf("navigation settings=%#v", got)
	}
	words := catalogPolicy(t, catalog, WordsPolicyID)
	if got := len(words.Commands); got != len(AllWordCommands) {
		t.Fatalf("word commands=%d", got)
	}
}

func TestEditorInvokerTypesMarkersThroughKernelDispatch(t *testing.T) {
	t.Parallel()
	document, node := emptyBlockDocument(t)
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	program, err := kernel.LinkConfiguredEventProgram(catalog, host.Prepared())
	if err != nil {
		t.Fatal(err)
	}
	engine := kernel.NewEngine(document, kernel.NewResourceStore(nil))
	invoker := &EditorInvoker{}
	dispatch := func(text string, start, end int) kernel.DispatchOutcome {
		t.Helper()
		outcome := kernel.DispatchOutcome{}
		if _, err := kernel.Transact(engine, func(draft *kernel.EngineDraft) (struct{}, error) {
			target := kernel.NewTextTarget(
				node,
				model.NewUTF16Range(model.UTF16Offset(start), model.UTF16Offset(end)),
				engine.Revision(),
			)
			event := kernel.NewTextEvent(kernel.NewTextInput(target, text), kernel.EventOriginTyping)
			var dispatchErr error
			outcome, dispatchErr = program.DispatchEngine(event, draft, invoker)
			return struct{}{}, dispatchErr
		}); err != nil {
			t.Fatal(err)
		}
		return outcome
	}
	for _, text := range []string{"*", "*", "word"} {
		if outcome := dispatch(text, 0, 0); outcome.Status != kernel.OutcomeHandled {
			t.Fatalf("typing %q outcome=%#v", text, outcome)
		}
	}
	current, _ := engine.Snapshot().Document().Node(node)
	if current.Text().String() != "word" || len(current.Marks()) != 1 ||
		current.Marks()[0].Range() != model.NewUTF16Range(0, 4) ||
		current.Marks()[0].Level() != 2 {
		t.Fatalf("node text=%q marks=%#v", current.Text().String(), current.Marks())
	}
	revision := engine.Revision()
	if outcome := dispatch("*", 4, 4); outcome.Status != kernel.OutcomeHandled {
		t.Fatalf("exit outcome=%#v", outcome)
	}
	if engine.Revision() != revision {
		t.Fatal("marker exit published a revision")
	}
	if _, ok := invoker.ActiveMark(); ok {
		t.Fatal("marker exit retained active mark")
	}
}

func TestEditorInvokerEditsGeneratedMarkersAtomically(t *testing.T) {
	t.Parallel()
	document, node := emptyBlockDocument(t)
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, "ab"); err != nil {
			return err
		}
		_, err := draft.AddMark(node, model.NewUTF16Range(0, 2), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	program, err := kernel.LinkConfiguredEventProgram(catalog, host.Prepared())
	if err != nil {
		t.Fatal(err)
	}
	engine := kernel.NewEngine(document, kernel.NewResourceStore(nil))
	invoker := &EditorInvoker{}
	dispatch := func(text string) error {
		t.Helper()
		_, err := kernel.Transact(engine, func(draft *kernel.EngineDraft) (struct{}, error) {
			target := kernel.NewAttributeTarget(
				node,
				MarkerKeyForMark(1),
				engine.Revision(),
			)
			event := kernel.NewAttributeEvent(
				kernel.NewAttributeInput(target, text),
				kernel.EventOriginTyping,
			)
			outcome, err := program.DispatchEngine(event, draft, invoker)
			if err == nil && outcome.Status != kernel.OutcomeHandled {
				t.Fatalf("attribute input %q outcome=%#v", text, outcome)
			}
			return struct{}{}, err
		})
		return err
	}
	before := engine.Snapshot()
	for _, invalid := range []string{"***", "*_", "_*"} {
		if err := dispatch(invalid); err == nil {
			t.Fatalf("invalid marker input %q was accepted", invalid)
		}
		if engine.Revision() != before.Revision() {
			t.Fatalf("invalid marker input %q published a revision", invalid)
		}
		current, _ := engine.Snapshot().Document().Node(node)
		if len(current.Marks()) != 1 || current.Marks()[0].Level() != 2 {
			t.Fatalf("invalid marker input %q changed marks=%#v", invalid, current.Marks())
		}
	}
	if err := dispatch("*"); err != nil {
		t.Fatal(err)
	}
	current, _ := engine.Snapshot().Document().Node(node)
	if len(current.Marks()) != 1 || current.Marks()[0].Level() != 1 {
		t.Fatalf("weak marker level marks=%#v", current.Marks())
	}
	if err := dispatch(""); err != nil {
		t.Fatal(err)
	}
	current, _ = engine.Snapshot().Document().Node(node)
	if len(current.Marks()) != 0 || current.Text().String() != "ab" {
		t.Fatalf("marker deletion text=%q marks=%#v", current.Text().String(), current.Marks())
	}
}

func assertRuleOrder(
	t *testing.T,
	rule kernel.RuleDescriptor,
	order uint32,
	phase kernel.RulePhaseKind,
	trigger string,
) {
	t.Helper()
	if rule.Order != order || rule.Phase.Kind != phase ||
		rule.Trigger.Pattern() != trigger {
		t.Fatalf(
			"rule=%s order=%d phase=%v trigger=%q want %d %v %q",
			rule.ID,
			rule.Order,
			rule.Phase.Kind,
			rule.Trigger.Pattern(),
			order,
			phase,
			trigger,
		)
	}
}

func assertAttributeSegment(
	t *testing.T,
	segment projection.Segment,
	key string,
	start, end model.UTF16Offset,
) {
	t.Helper()
	attribute, ok := segment.AttributeTarget()
	if !ok || attribute.Key() != key ||
		segment.ProjectedRange() != model.NewUTF16Range(start, end) {
		t.Fatalf("attribute segment=%#v target=%#v", segment, attribute)
	}
}

func assertEditableSegment(
	t *testing.T,
	segment projection.Segment,
	start, end model.UTF16Offset,
) {
	t.Helper()
	if _, ok := segment.SemanticTarget(); !ok || segment.Kind() != projection.SegmentEditable ||
		segment.ProjectedRange() != model.NewUTF16Range(start, end) {
		t.Fatalf("editable segment=%#v", segment)
	}
}

func catalogPolicy(
	t *testing.T,
	catalog kernel.PolicyCatalog,
	id kernel.PolicyID,
) kernel.PolicyDescriptor {
	t.Helper()
	policy, ok := catalog.Policy(id)
	if !ok {
		t.Fatalf("missing policy %s", id)
	}
	return policy
}

func settingIDs(settings []kernel.SettingDescriptor) []kernel.SettingID {
	ids := make([]kernel.SettingID, 0, len(settings))
	for _, setting := range settings {
		ids = append(ids, setting.ID)
	}
	return ids
}

func commandIDs(policy kernel.PolicyDescriptor) []kernel.CommandID {
	return append([]kernel.CommandID(nil), policy.Commands...)
}

func markupConfiguration(
	t *testing.T,
	enabled bool,
) (*kernel.PreparedPolicyConfiguration, error) {
	t.Helper()
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		return nil, err
	}
	host, err := kernel.NewPolicyHost(catalog)
	if err != nil {
		return nil, err
	}
	if err := host.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(WritingPolicyID, enabled)
		return nil
	}); err != nil {
		return nil, err
	}
	return host.PreparedSnapshot(), nil
}
