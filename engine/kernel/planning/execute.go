package planning

import "fmt"

// Checkpoint is a journal that can restore the state owned by ordered choice.
// M is the journal's typed mark.
type Checkpoint[M any] interface {
	Checkpoint() M
	Rollback(M)
}

// Execute runs a control program without fallible callbacks. A top-level Pass
// does not roll the journal back; only an enclosing ordered choice does.
func Execute[C any, M any, J Checkpoint[M]](
	program *ControlProgram,
	journal J,
	context *C,
	invoke func(LeafID, *C, J) Outcome,
) Outcome {
	outcome, err := runIterative(program, journal, context, func(leaf LeafID, context *C, journal J) (Outcome, error) {
		return invoke(leaf, context, journal), nil
	})
	if err != nil {
		panic("planning: infallible control execution returned an error")
	}
	return outcome
}

// TryExecute runs a control program with fallible callbacks. A callback error
// stops all execution and restores the journal to its execution-entry state.
func TryExecute[C any, M any, J Checkpoint[M]](
	program *ControlProgram,
	journal J,
	context *C,
	invoke func(LeafID, *C, J) (Outcome, error),
) (Outcome, error) {
	mark := journal.Checkpoint()
	outcome, err := runIterative(program, journal, context, invoke)
	if err != nil {
		journal.Rollback(mark)
	}
	return outcome, err
}

func runIterative[C any, M any, J Checkpoint[M]](
	program *ControlProgram,
	journal J,
	context *C,
	invoke func(LeafID, *C, J) (Outcome, error),
) (Outcome, error) {
	current := program.root
	hasCurrent := true
	var result Outcome
	hasResult := false
	var frames []executionFrame[M]

	for {
		if hasCurrent {
			hasCurrent = false
			node := program.node(current)
			switch node.kind {
			case controlIdentity:
				result = OutcomeHandled
				hasResult = true
			case controlPass:
				result = OutcomePass
				hasResult = true
			case controlLeaf:
				outcome, err := invoke(node.leaf, context, journal)
				if err != nil {
					return 0, err
				}
				result = checkedOutcome(outcome)
				hasResult = true
			case controlThen:
				frames = append(frames, executionFrame[M]{
					kind:     controlThen,
					children: node.children,
					next:     1,
				})
				current = node.children[0]
				hasCurrent = true
			case controlFirst:
				frames = append(frames, executionFrame[M]{
					kind:     controlFirst,
					children: node.children,
					next:     1,
					mark:     journal.Checkpoint(),
				})
				current = node.children[0]
				hasCurrent = true
			default:
				panic("planning: invalid control node")
			}
			continue
		}

		if !hasResult {
			return OutcomePass, nil
		}
		if len(frames) == 0 {
			return result, nil
		}

		frame := &frames[len(frames)-1]
		switch frame.kind {
		case controlThen:
			switch checkedOutcome(result) {
			case OutcomePass:
				frames = frames[:len(frames)-1]
				result = OutcomePass
			case OutcomeHandled:
				if frame.next < len(frame.children) {
					current = frame.children[frame.next]
					frame.next++
					hasCurrent = true
				} else {
					frames = frames[:len(frames)-1]
					result = OutcomeHandled
				}
			}
		case controlFirst:
			switch checkedOutcome(result) {
			case OutcomeHandled:
				frames = frames[:len(frames)-1]
				result = OutcomeHandled
			case OutcomePass:
				journal.Rollback(frame.mark)
				if frame.next < len(frame.children) {
					current = frame.children[frame.next]
					frame.next++
					hasCurrent = true
				} else {
					frames = frames[:len(frames)-1]
					result = OutcomePass
				}
			}
		default:
			panic("planning: invalid control frame")
		}
	}
}

type executionFrame[M any] struct {
	kind     controlKind
	children []ControlID
	next     int
	mark     M
}

func checkedOutcome(outcome Outcome) Outcome {
	switch outcome {
	case OutcomeHandled, OutcomePass:
		return outcome
	default:
		panic(fmt.Sprintf("planning: invalid outcome %d", outcome))
	}
}
