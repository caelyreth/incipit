package planning

import (
	"errors"
	"reflect"
	"testing"
)

func TestDefaultControlPlanBudget(t *testing.T) {
	t.Parallel()
	if DefaultControlPlanBudget != 65_536 {
		t.Fatalf("DefaultControlPlanBudget = %d, want 65536", DefaultControlPlanBudget)
	}
}

func TestPrepareCompilesContinuations(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	first := builder.Leaf(NewLeafID(1))
	second := builder.Leaf(NewLeafID(2))
	fallback := builder.Leaf(NewLeafID(3))
	program := mustBuild(t, &builder, builder.Then(first, builder.First(second, fallback)))
	prepared := program.Prepare(128)

	if got := prepared.Kind(); got != ControlPlanCompiled {
		t.Fatalf("Kind() = %v, want compiled", got)
	}
	if got := prepared.CheckpointCount(); got != 1 {
		t.Fatalf("CheckpointCount() = %d, want 1", got)
	}
	if got := prepared.InstructionCount(); got <= 0 {
		t.Fatalf("InstructionCount() = %d, want positive", got)
	}
	if got := prepared.InstructionCount(); got != len(prepared.instructions) {
		t.Fatalf("InstructionCount() = %d, instructions = %d", got, len(prepared.instructions))
	}
}

func TestPrepareUsesDefaultBudget(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	program := mustBuild(t, &builder, builder.Leaf(NewLeafID(1)))
	prepared := program.Prepare(DefaultControlPlanBudget)
	if got := prepared.Kind(); got != ControlPlanCompiled {
		t.Fatalf("Kind() = %v, want compiled", got)
	}
}

func TestPreparationBudgetFallsBackToIterative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		budget int
	}{
		{name: "zero", budget: 0},
		{name: "negative", budget: -1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var builder ControlBuilder
			program := mustBuild(t, &builder, builder.Leaf(NewLeafID(1)))
			prepared := program.Prepare(test.budget)
			if got := prepared.Kind(); got != ControlPlanIterativeFallback {
				t.Fatalf("Kind() = %v, want iterative fallback", got)
			}
			if prepared.InstructionCount() != 0 || prepared.CheckpointCount() != 0 {
				t.Fatalf("fallback counts = %d/%d, want 0/0", prepared.InstructionCount(), prepared.CheckpointCount())
			}
			if prepared.iterative != program {
				t.Fatal("fallback did not retain original program")
			}
			journal := &testJournal{}
			outcome := ExecutePrepared(&prepared, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
				journal.values = append(journal.values, leaf.Value())
				return OutcomeHandled
			})
			if outcome != OutcomeHandled || !reflect.DeepEqual(journal.values, []uint32{1}) {
				t.Fatalf("fallback execution = %v, %v", outcome, journal.values)
			}
		})
	}
}

func TestPreparationBudgetBoundary(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	first := builder.Leaf(NewLeafID(1))
	second := builder.Leaf(NewLeafID(2))
	program := mustBuild(t, &builder, builder.First(first, second))
	preparedThree := program.Prepare(3)
	if got := preparedThree.Kind(); got != ControlPlanCompiled {
		t.Fatalf("budget 3 kind = %v, want compiled", got)
	}
	preparedTwo := program.Prepare(2)
	if got := preparedTwo.Kind(); got != ControlPlanIterativeFallback {
		t.Fatalf("budget 2 kind = %v, want fallback", got)
	}
}

func TestPreparedMatchesIterativeExecution(t *testing.T) {
	t.Parallel()

	type program struct {
		name   string
		build  func(*ControlBuilder) ControlID
		leaves int
	}
	programs := []program{
		{
			name:   "single leaf",
			build:  func(b *ControlBuilder) ControlID { return b.Leaf(NewLeafID(1)) },
			leaves: 1,
		},
		{
			name: "top-level then pass",
			build: func(b *ControlBuilder) ControlID {
				return b.Then(b.Leaf(NewLeafID(1)), b.Pass())
			},
			leaves: 1,
		},
		{
			name: "flat choice",
			build: func(b *ControlBuilder) ControlID {
				return b.First(b.Leaf(NewLeafID(1)), b.Leaf(NewLeafID(2)))
			},
			leaves: 2,
		},
		{
			name: "then containing choice",
			build: func(b *ControlBuilder) ControlID {
				return b.Then(
					b.Leaf(NewLeafID(1)),
					b.First(b.Leaf(NewLeafID(2)), b.Leaf(NewLeafID(3))),
				)
			},
			leaves: 3,
		},
		{
			name: "choice of sequences",
			build: func(b *ControlBuilder) ControlID {
				return b.First(
					b.Then(b.Leaf(NewLeafID(1)), b.Leaf(NewLeafID(2))),
					b.Then(b.Leaf(NewLeafID(3)), b.Leaf(NewLeafID(4))),
				)
			},
			leaves: 4,
		},
		{
			name: "nested choices",
			build: func(b *ControlBuilder) ControlID {
				return b.First(
					b.Leaf(NewLeafID(1)),
					b.First(b.Leaf(NewLeafID(2)), b.Leaf(NewLeafID(3))),
				)
			},
			leaves: 3,
		},
		{
			name: "choice after choice",
			build: func(b *ControlBuilder) ControlID {
				firstChoice := b.First(b.Leaf(NewLeafID(1)), b.Leaf(NewLeafID(2)))
				secondChoice := b.First(b.Leaf(NewLeafID(3)), b.Leaf(NewLeafID(4)))
				return b.Then(firstChoice, secondChoice)
			},
			leaves: 4,
		},
		{
			name: "choice containing sequence containing choice",
			build: func(b *ControlBuilder) ControlID {
				inner := b.First(b.Leaf(NewLeafID(3)), b.Leaf(NewLeafID(4)))
				sequence := b.Then(b.Leaf(NewLeafID(1)), inner)
				return b.First(sequence, b.Leaf(NewLeafID(5)))
			},
			leaves: 5,
		},
	}

	for _, test := range programs {
		test := test
		t.Run(test.name, func(t *testing.T) {
			var builder ControlBuilder
			control := test.build(&builder)
			program := mustBuild(t, &builder, control)
			prepared := program.Prepare(DefaultControlPlanBudget)
			if prepared.Kind() != ControlPlanCompiled {
				t.Fatalf("prepare kind = %v, want compiled", prepared.Kind())
			}
			for mask := 0; mask < 1<<test.leaves; mask++ {
				outcomes := make(map[LeafID]Outcome, test.leaves)
				for index := 0; index < test.leaves; index++ {
					outcome := OutcomeHandled
					if mask&(1<<index) != 0 {
						outcome = OutcomePass
					}
					outcomes[NewLeafID(uint32(index+1))] = outcome
				}
				compareExecution(t, program, &prepared, outcomes)
			}
		})
	}
}

func TestPreparedFallbackMatchesIterativeForExhaustedBudget(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	program := mustBuild(t, &builder, builder.First(
		builder.Then(builder.Leaf(NewLeafID(1)), builder.Leaf(NewLeafID(2))),
		builder.Leaf(NewLeafID(3)),
	))
	prepared := program.Prepare(0)
	for mask := 0; mask < 8; mask++ {
		outcomes := map[LeafID]Outcome{
			1: outcomeFromBit(mask, 0),
			2: outcomeFromBit(mask, 1),
			3: outcomeFromBit(mask, 2),
		}
		compareExecution(t, program, &prepared, outcomes)
	}
}

func TestPreparedTryExecuteRollsBackOnError(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	program := mustBuild(t, &builder, builder.First(
		builder.Then(builder.Leaf(NewLeafID(1)), builder.Leaf(NewLeafID(2))),
		builder.Leaf(NewLeafID(3)),
	))
	for _, prepared := range []PreparedControl{
		program.Prepare(DefaultControlPlanBudget),
		program.Prepare(0),
	} {
		journal := &testJournal{values: []uint32{99}}
		rejection := errors.New("rejected")
		_, err := TryExecutePrepared(&prepared, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) (Outcome, error) {
			journal.values = append(journal.values, leaf.Value())
			if leaf == NewLeafID(2) {
				return 0, rejection
			}
			return OutcomeHandled, nil
		})
		if !errors.Is(err, rejection) {
			t.Fatalf("%v: error = %v, want rejection", prepared.Kind(), err)
		}
		if want := []uint32{99}; !reflect.DeepEqual(journal.values, want) {
			t.Fatalf("%v: journal = %v, want %v", prepared.Kind(), journal.values, want)
		}
	}
}

func TestPreparedTopLevelPassDoesNotRollback(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	program := mustBuild(t, &builder, builder.Then(builder.Leaf(NewLeafID(1)), builder.Pass()))
	prepared := program.Prepare(DefaultControlPlanBudget)
	journal := &testJournal{}
	outcome := ExecutePrepared(&prepared, journal, &struct{}{}, func(_ LeafID, _ *struct{}, journal *testJournal) Outcome {
		journal.values = append(journal.values, 7)
		return OutcomeHandled
	})
	if outcome != OutcomePass {
		t.Fatalf("outcome = %v, want Pass", outcome)
	}
	if want := []uint32{7}; !reflect.DeepEqual(journal.values, want) {
		t.Fatalf("journal = %v, want %v", journal.values, want)
	}
}

func TestPrepareCompilerShape(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	a := builder.Leaf(NewLeafID(1))
	b := builder.Leaf(NewLeafID(2))
	program := mustBuild(t, &builder, builder.First(a, b))
	prepared := program.Prepare(DefaultControlPlanBudget)

	want := []instruction{
		{kind: instructionHalt, outcome: OutcomeHandled},
		{kind: instructionHalt, outcome: OutcomePass},
		{kind: instructionRestore, slot: 0, next: 1},
		{kind: instructionInvoke, leaf: 2, handled: 0, passed: 2},
		{kind: instructionRestore, slot: 0, next: 3},
		{kind: instructionInvoke, leaf: 1, handled: 0, passed: 4},
		{kind: instructionCheckpoint, slot: 0, next: 5},
	}
	if !reflect.DeepEqual(prepared.instructions, want) {
		t.Fatalf("instructions:\n got %+v\nwant %+v", prepared.instructions, want)
	}
	if prepared.entry != 6 {
		t.Fatalf("entry = %d, want 6", prepared.entry)
	}
}

func compareExecution(t *testing.T, program *ControlProgram, prepared *PreparedControl, outcomes map[LeafID]Outcome) {
	t.Helper()
	iterativeJournal := &testJournal{}
	var iterativeCalls []LeafID
	iterativeOutcome := Execute(program, iterativeJournal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		iterativeCalls = append(iterativeCalls, leaf)
		journal.values = append(journal.values, leaf.Value())
		return outcomes[leaf]
	})
	preparedJournal := &testJournal{}
	var preparedCalls []LeafID
	preparedOutcome := ExecutePrepared(prepared, preparedJournal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		preparedCalls = append(preparedCalls, leaf)
		journal.values = append(journal.values, leaf.Value())
		return outcomes[leaf]
	})
	if preparedOutcome != iterativeOutcome {
		t.Fatalf("outcomes differ: prepared=%v iterative=%v", preparedOutcome, iterativeOutcome)
	}
	if !reflect.DeepEqual(preparedCalls, iterativeCalls) {
		t.Fatalf("calls differ: prepared=%v iterative=%v", preparedCalls, iterativeCalls)
	}
	if !reflect.DeepEqual(preparedJournal.values, iterativeJournal.values) {
		t.Fatalf("journals differ: prepared=%v iterative=%v", preparedJournal.values, iterativeJournal.values)
	}
}

func outcomeFromBit(mask, bit int) Outcome {
	if mask&(1<<bit) != 0 {
		return OutcomePass
	}
	return OutcomeHandled
}
