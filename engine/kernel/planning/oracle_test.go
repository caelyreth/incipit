package planning

import (
	"fmt"
	"reflect"
	"testing"
)

// recursiveOutcome is an independent specification of the control algebra. It
// intentionally does not share execution machinery with the iterative or
// compiled evaluators.
func recursiveOutcome(
	program *ControlProgram,
	id ControlID,
	journal *testJournal,
	outcomes map[LeafID]Outcome,
) Outcome {
	node := program.node(id)
	switch node.kind {
	case controlIdentity:
		return OutcomeHandled
	case controlPass:
		return OutcomePass
	case controlLeaf:
		journal.values = append(journal.values, node.leaf.Value())
		return outcomes[node.leaf]
	case controlThen:
		for _, child := range node.children {
			if recursiveOutcome(program, child, journal, outcomes) == OutcomePass {
				return OutcomePass
			}
		}
		return OutcomeHandled
	case controlFirst:
		mark := journal.Checkpoint()
		for index, child := range node.children {
			if recursiveOutcome(program, child, journal, outcomes) == OutcomeHandled {
				return OutcomeHandled
			}
			journal.Rollback(mark)
			if index+1 == len(node.children) {
				return OutcomePass
			}
		}
		return OutcomePass
	default:
		panic("invalid oracle node")
	}
}

func TestGeneratedProgramsMatchRecursiveOracle(t *testing.T) {
	t.Parallel()

	sequences := []int{1, 2, 3, 4, 5}
	for _, leafCount := range sequences {
		leafCount := leafCount
		t.Run(fmt.Sprintf("leaves-%d", leafCount), func(t *testing.T) {
			t.Parallel()
			var builder ControlBuilder
			leaves := make([]ControlID, leafCount)
			for index := range leaves {
				leaves[index] = builder.Leaf(NewLeafID(uint32(index + 1)))
			}
			shapes := map[string]ControlID{
				"then":           builder.Then(leaves...),
				"first":          builder.First(leaves...),
				"alternating":    alternatingControl(&builder, leaves),
				"nested-both":    nestedBothControl(&builder, leaves),
				"choice-chain":   choiceChainControl(&builder, leaves),
				"all-identities": builder.Then(builder.Identity(), builder.Identity()),
				"all-passes":     builder.First(builder.Pass(), builder.Pass()),
			}
			programs := make(map[string]*ControlProgram, len(shapes))
			for name, root := range shapes {
				programs[name] = mustBuild(t, &builder, root)
			}

			compiled := make(map[string]*PreparedControl, len(programs))
			fallback := make(map[string]*PreparedControl, len(programs))
			for name, program := range programs {
				compiledValue := program.Prepare(DefaultControlPlanBudget)
				fallbackValue := program.Prepare(0)
				compiled[name] = &compiledValue
				fallback[name] = &fallbackValue
			}

			for mask := 0; mask < 1<<leafCount; mask++ {
				outcomes := make(map[LeafID]Outcome, leafCount)
				for index := 0; index < leafCount; index++ {
					outcomes[NewLeafID(uint32(index+1))] = outcomeFromBit(mask, index)
				}
				for name, program := range programs {
					oracleJournal := &testJournal{values: []uint32{99}}
					oracle := recursiveOutcome(program, program.root, oracleJournal, outcomes)
					assertEquivalent(t, mask, name, "iterative", program, oracle, oracleJournal.values, outcomes)
					assertPreparedEquivalent(t, mask, name, "compiled", compiled[name], oracle, oracleJournal.values, outcomes)
					assertPreparedEquivalent(t, mask, name, "fallback", fallback[name], oracle, oracleJournal.values, outcomes)
				}
			}
		})
	}
}

func alternatingControl(builder *ControlBuilder, leaves []ControlID) ControlID {
	if len(leaves) == 1 {
		return leaves[0]
	}
	left := builder.Then(leaves[:2]...)
	if len(leaves) == 2 {
		return left
	}
	return builder.First(left, alternatingControl(builder, leaves[2:]))
}

func nestedBothControl(builder *ControlBuilder, leaves []ControlID) ControlID {
	if len(leaves) == 1 {
		return leaves[0]
	}
	middle := len(leaves) / 2
	return builder.First(
		builder.Then(leaves[:middle]...),
		builder.Then(leaves[middle:]...),
	)
}

func choiceChainControl(builder *ControlBuilder, leaves []ControlID) ControlID {
	root := leaves[len(leaves)-1]
	for index := len(leaves) - 2; index >= 0; index-- {
		root = builder.First(leaves[index], root)
	}
	return root
}

func assertEquivalent(
	t *testing.T,
	mask int,
	name string,
	representation string,
	program *ControlProgram,
	wantOutcome Outcome,
	wantJournal []uint32,
	outcomes map[LeafID]Outcome,
) {
	t.Helper()
	journal := &testJournal{values: []uint32{99}}
	outcome := Execute(program, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		journal.values = append(journal.values, leaf.Value())
		return outcomes[leaf]
	})
	if outcome != wantOutcome || !reflect.DeepEqual(journal.values, wantJournal) {
		t.Fatalf(
			"mask %04b %s/%s: outcome=%v journal=%v, want outcome=%v journal=%v",
			mask, name, representation, outcome, journal.values, wantOutcome, wantJournal,
		)
	}
}

func assertPreparedEquivalent(
	t *testing.T,
	mask int,
	name string,
	representation string,
	prepared *PreparedControl,
	wantOutcome Outcome,
	wantJournal []uint32,
	outcomes map[LeafID]Outcome,
) {
	t.Helper()
	journal := &testJournal{values: []uint32{99}}
	outcome := ExecutePrepared(prepared, journal, &struct{}{}, func(leaf LeafID, _ *struct{}, journal *testJournal) Outcome {
		journal.values = append(journal.values, leaf.Value())
		return outcomes[leaf]
	})
	if outcome != wantOutcome || !reflect.DeepEqual(journal.values, wantJournal) {
		t.Fatalf(
			"mask %04b %s/%s: outcome=%v journal=%v, want outcome=%v journal=%v",
			mask, name, representation, outcome, journal.values, wantOutcome, wantJournal,
		)
	}
}
