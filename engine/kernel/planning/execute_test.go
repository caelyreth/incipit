package planning

import (
	"errors"
	"reflect"
	"testing"
)

type testJournal struct {
	values []uint32
}

func (j *testJournal) Checkpoint() int { return len(j.values) }

func (j *testJournal) Rollback(mark int) {
	j.values = j.values[:mark]
}

type executionTrace struct {
	outcome Outcome
	calls   []LeafID
	values  []uint32
}

func runTrace(t *testing.T, program *ControlProgram, outcomes map[LeafID]Outcome) executionTrace {
	t.Helper()
	journal := &testJournal{}
	var calls []LeafID
	outcome := Execute(program, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		calls = append(calls, leaf)
		journal.values = append(journal.values, leaf.Value())
		return outcomes[leaf]
	})
	return executionTrace{outcome: outcome, calls: calls, values: journal.values}
}

func mustBuild(t *testing.T, builder *ControlBuilder, root ControlID) *ControlProgram {
	t.Helper()
	program, err := builder.Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return program
}

func TestThenTruthTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		outcomes []Outcome
		want     Outcome
		calls    []LeafID
	}{
		{
			name:     "all handled",
			outcomes: []Outcome{OutcomeHandled, OutcomeHandled, OutcomeHandled},
			want:     OutcomeHandled,
			calls:    []LeafID{1, 2, 3},
		},
		{
			name:     "pass at first",
			outcomes: []Outcome{OutcomePass, OutcomeHandled, OutcomeHandled},
			want:     OutcomePass,
			calls:    []LeafID{1},
		},
		{
			name:     "pass in middle",
			outcomes: []Outcome{OutcomeHandled, OutcomePass, OutcomeHandled},
			want:     OutcomePass,
			calls:    []LeafID{1, 2},
		},
		{
			name:     "pass at last",
			outcomes: []Outcome{OutcomeHandled, OutcomeHandled, OutcomePass},
			want:     OutcomePass,
			calls:    []LeafID{1, 2, 3},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var builder ControlBuilder
			leaves := []ControlID{
				builder.Leaf(NewLeafID(1)),
				builder.Leaf(NewLeafID(2)),
				builder.Leaf(NewLeafID(3)),
			}
			program := mustBuild(t, &builder, builder.Then(leaves...))
			outcomes := map[LeafID]Outcome{
				1: test.outcomes[0],
				2: test.outcomes[1],
				3: test.outcomes[2],
			}
			trace := runTrace(t, program, outcomes)
			if trace.outcome != test.want || !reflect.DeepEqual(trace.calls, test.calls) {
				t.Fatalf("trace = %+v, want outcome %v calls %v", trace, test.want, test.calls)
			}
			if !reflect.DeepEqual(trace.values, leafValues(test.calls)) {
				t.Fatalf("journal = %v, want %v", trace.values, leafValues(test.calls))
			}
		})
	}
}

func TestFirstTruthTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		outcomes []Outcome
		want     Outcome
		calls    []LeafID
		values   []uint32
	}{
		{
			name:     "first handled commits",
			outcomes: []Outcome{OutcomeHandled, OutcomeHandled},
			want:     OutcomeHandled,
			calls:    []LeafID{1},
			values:   []uint32{1},
		},
		{
			name:     "pass then handled",
			outcomes: []Outcome{OutcomePass, OutcomeHandled},
			want:     OutcomeHandled,
			calls:    []LeafID{1, 2},
			values:   []uint32{2},
		},
		{
			name:     "all pass",
			outcomes: []Outcome{OutcomePass, OutcomePass},
			want:     OutcomePass,
			calls:    []LeafID{1, 2},
			values:   nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var builder ControlBuilder
			first := builder.Leaf(NewLeafID(1))
			second := builder.Leaf(NewLeafID(2))
			program := mustBuild(t, &builder, builder.First(first, second))
			outcomes := map[LeafID]Outcome{1: test.outcomes[0], 2: test.outcomes[1]}
			trace := runTrace(t, program, outcomes)
			if trace.outcome != test.want || !reflect.DeepEqual(trace.calls, test.calls) {
				t.Fatalf("trace = %+v, want outcome %v calls %v", trace, test.want, test.calls)
			}
			if !sameUint32s(trace.values, test.values) {
				t.Fatalf("journal = %v, want %v", trace.values, test.values)
			}
		})
	}
}

func TestTopLevelPassDoesNotRollback(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	write := builder.Leaf(NewLeafID(1))
	program := mustBuild(t, &builder, builder.Then(write, builder.Pass()))
	journal := &testJournal{}
	outcome := Execute(program, journal, &struct{}{}, func(_ LeafID, _ *struct{}, journal *testJournal) Outcome {
		journal.values = append(journal.values, 1, 99)
		return OutcomeHandled
	})
	if outcome != OutcomePass {
		t.Fatalf("outcome = %v, want Pass", outcome)
	}
	if want := []uint32{1, 99}; !reflect.DeepEqual(journal.values, want) {
		t.Fatalf("journal = %v, want %v", journal.values, want)
	}
}

func TestChoiceRollsBackEveryPassedBranch(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	first := builder.Leaf(NewLeafID(1))
	second := builder.Leaf(NewLeafID(2))
	third := builder.Leaf(NewLeafID(3))
	program := mustBuild(t, &builder, builder.First(first, second, third))
	journal := &testJournal{values: []uint32{99}}
	var calls []LeafID
	outcome := Execute(program, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		calls = append(calls, leaf)
		journal.values = append(journal.values, leaf.Value())
		if leaf == NewLeafID(3) {
			return OutcomeHandled
		}
		return OutcomePass
	})
	if outcome != OutcomeHandled {
		t.Fatalf("outcome = %v, want Handled", outcome)
	}
	if want := []LeafID{1, 2, 3}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if want := []uint32{99, 3}; !reflect.DeepEqual(journal.values, want) {
		t.Fatalf("journal = %v, want %v", journal.values, want)
	}
}

func TestNestedChoiceRollbackUsesBranchBoundary(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	outerWrite := builder.Leaf(NewLeafID(1))
	innerWrite := builder.Leaf(NewLeafID(2))
	fallback := builder.Leaf(NewLeafID(3))
	inner := builder.First(innerWrite, fallback)
	program := mustBuild(t, &builder, builder.Then(outerWrite, inner))
	journal := &testJournal{}
	outcome := Execute(program, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		journal.values = append(journal.values, leaf.Value())
		if leaf == 1 || leaf == 3 {
			return OutcomeHandled
		}
		return OutcomePass
	})
	if outcome != OutcomeHandled {
		t.Fatalf("outcome = %v, want Handled", outcome)
	}
	if want := []uint32{1, 3}; !reflect.DeepEqual(journal.values, want) {
		t.Fatalf("journal = %v, want %v", journal.values, want)
	}
}

func TestTryExecuteRollsBackOnError(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	successful := builder.Leaf(NewLeafID(1))
	rejected := builder.Leaf(NewLeafID(2))
	fallback := builder.Leaf(NewLeafID(3))
	program := mustBuild(t, &builder, builder.First(builder.Then(successful, rejected), fallback))
	rejection := errors.New("rejected")
	journal := &testJournal{values: []uint32{99}}
	var calls []LeafID
	outcome, err := TryExecute(program, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) (Outcome, error) {
		calls = append(calls, leaf)
		journal.values = append(journal.values, leaf.Value())
		if leaf == NewLeafID(2) {
			return 0, rejection
		}
		return OutcomeHandled, nil
	})
	if !errors.Is(err, rejection) {
		t.Fatalf("error = %v, want %v", err, rejection)
	}
	if outcome != 0 {
		t.Fatalf("outcome = %v, want zero", outcome)
	}
	if want := []LeafID{1, 2}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if want := []uint32{99}; !reflect.DeepEqual(journal.values, want) {
		t.Fatalf("journal = %v, want %v", journal.values, want)
	}
}

func TestTryExecuteSuccessKeepsTopLevelPassEffects(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	write := builder.Leaf(NewLeafID(1))
	program := mustBuild(t, &builder, builder.Then(write, builder.Pass()))
	journal := &testJournal{}
	outcome, err := TryExecute(program, journal, &struct{}{}, func(_ LeafID, _ *struct{}, journal *testJournal) (Outcome, error) {
		journal.values = append(journal.values, 7)
		return OutcomeHandled, nil
	})
	if err != nil {
		t.Fatalf("TryExecute() error = %v", err)
	}
	if outcome != OutcomePass {
		t.Fatalf("outcome = %v, want Pass", outcome)
	}
	if want := []uint32{7}; !reflect.DeepEqual(journal.values, want) {
		t.Fatalf("journal = %v, want %v", journal.values, want)
	}
}

func TestThenStopsAtPassInFiftyFourCombinations(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	leaves := []ControlID{
		builder.Leaf(NewLeafID(10)),
		builder.Leaf(NewLeafID(20)),
		builder.Leaf(NewLeafID(30)),
	}
	program := mustBuild(t, &builder, builder.Then(leaves...))

	for mask := 0; mask < 1<<len(leaves); mask++ {
		outcomes := make(map[LeafID]Outcome, len(leaves))
		var wantCalls []LeafID
		wantOutcome := OutcomeHandled
		for index, id := range leaves {
			leaf := program.node(id).leaf
			if mask&(1<<index) != 0 {
				outcomes[leaf] = OutcomePass
			} else {
				outcomes[leaf] = OutcomeHandled
			}
			wantCalls = append(wantCalls, leaf)
			if outcomes[leaf] == OutcomePass {
				wantOutcome = OutcomePass
				break
			}
		}
		trace := runTrace(t, program, outcomes)
		if trace.outcome != wantOutcome || !reflect.DeepEqual(trace.calls, wantCalls) {
			t.Fatalf("mask %03b: trace = %+v, want outcome %v calls %v", mask, trace, wantOutcome, wantCalls)
		}
	}
}

func leafValues(leaves []LeafID) []uint32 {
	if len(leaves) == 0 {
		return nil
	}
	values := make([]uint32, len(leaves))
	for index, leaf := range leaves {
		values[index] = leaf.Value()
	}
	return values
}

func sameUint32s(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
