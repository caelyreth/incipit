package planning

// ControlPlanKind identifies the representation chosen by Prepare.
type ControlPlanKind uint8

const (
	// ControlPlanCompiled uses a prepared continuation instruction stream.
	ControlPlanCompiled ControlPlanKind = iota + 1
	// ControlPlanIterativeFallback retains the iterative evaluator.
	ControlPlanIterativeFallback
)

func (k ControlPlanKind) String() string {
	switch k {
	case ControlPlanCompiled:
		return "compiled"
	case ControlPlanIterativeFallback:
		return "iterative-fallback"
	default:
		return "unknown"
	}
}

// PreparedControl is an immutable, ready-to-execute representation of a
// control program.
type PreparedControl struct {
	kind             ControlPlanKind
	instructions     []instruction
	entry            int
	iterative        *ControlProgram
	instructionCount int
	checkpointCount  int
}

type instructionKind uint8

const (
	instructionInvoke instructionKind = iota + 1
	instructionCheckpoint
	instructionRestore
	instructionHalt
)

type instruction struct {
	kind    instructionKind
	leaf    LeafID
	handled int
	passed  int
	slot    int
	next    int
	outcome Outcome
}

// Prepare compiles known control structure into continuation instructions.
// Negative budgets behave like zero. If the traversal budget is exceeded, the
// prepared value retains the iterative evaluator and identical semantics.
func (p *ControlProgram) Prepare(budget int) PreparedControl {
	if budget < 0 {
		budget = 0
	}
	compiler := controlCompiler{
		program: p,
		budget:  budget,
	}
	handled := compiler.emit(instruction{kind: instructionHalt, outcome: OutcomeHandled})
	passed := compiler.emit(instruction{kind: instructionHalt, outcome: OutcomePass})
	entry, err := compiler.compile(p.root, handled, passed)
	if err != nil {
		if err == errPrepareBudget {
			return PreparedControl{
				kind:      ControlPlanIterativeFallback,
				iterative: p,
			}
		}
		panic("planning: validated control program contains an invalid node")
	}
	return PreparedControl{
		kind:             ControlPlanCompiled,
		instructions:     compiler.instructions,
		entry:            entry,
		instructionCount: len(compiler.instructions),
		checkpointCount:  compiler.checkpoints,
	}
}

// Kind returns the representation selected during preparation.
func (p *PreparedControl) Kind() ControlPlanKind { return p.kind }

// InstructionCount returns the total compiled instruction count, including both
// halt terminators.
func (p *PreparedControl) InstructionCount() int { return p.instructionCount }

// CheckpointCount returns the number of compiled checkpoint slots.
func (p *PreparedControl) CheckpointCount() int { return p.checkpointCount }

// ExecutePrepared runs a prepared plan without fallible callbacks. It retains
// the execution-entry checkpoint behavior of TryExecutePrepared.
func ExecutePrepared[C any, M any, J Checkpoint[M]](
	prepared *PreparedControl,
	journal J,
	context *C,
	invoke func(LeafID, *C, J) Outcome,
) Outcome {
	outcome, err := TryExecutePrepared(prepared, journal, context, func(leaf LeafID, context *C, journal J) (Outcome, error) {
		return invoke(leaf, context, journal), nil
	})
	if err != nil {
		panic("planning: infallible prepared execution returned an error")
	}
	return outcome
}

// TryExecutePrepared runs a prepared plan with fallible callbacks. Callback
// errors restore the journal to its execution-entry state.
func TryExecutePrepared[C any, M any, J Checkpoint[M]](
	prepared *PreparedControl,
	journal J,
	context *C,
	invoke func(LeafID, *C, J) (Outcome, error),
) (Outcome, error) {
	if prepared.kind == ControlPlanIterativeFallback {
		return TryExecute(prepared.iterative, journal, context, invoke)
	}
	if prepared.kind != ControlPlanCompiled {
		panic("planning: invalid prepared control representation")
	}

	mark := journal.Checkpoint()
	outcome, err := executeCompiled(prepared, journal, context, invoke)
	if err != nil {
		journal.Rollback(mark)
	}
	return outcome, err
}

type prepareError uint8

const (
	errPrepareBudget prepareError = iota + 1
	errPrepareInvalid
)

func (e prepareError) Error() string {
	switch e {
	case errPrepareBudget:
		return "control plan budget exceeded"
	case errPrepareInvalid:
		return "invalid control program"
	default:
		return "unknown control plan error"
	}
}

type controlCompiler struct {
	program      *ControlProgram
	instructions []instruction
	checkpoints  int
	steps        int
	budget       int
}

func (c *controlCompiler) compile(id ControlID, handled, passed int) (int, error) {
	c.steps = saturatingIncrement(c.steps)
	if c.steps > c.budget {
		return 0, errPrepareBudget
	}
	if int(id.Value()) >= len(c.program.nodes) {
		return 0, errPrepareInvalid
	}

	node := c.program.nodes[id.Value()]
	switch node.kind {
	case controlIdentity:
		return handled, nil
	case controlPass:
		return passed, nil
	case controlLeaf:
		return c.emit(instruction{
			kind:    instructionInvoke,
			leaf:    node.leaf,
			handled: handled,
			passed:  passed,
		}), nil
	case controlThen:
		entry := handled
		for index := len(node.children) - 1; index >= 0; index-- {
			var err error
			entry, err = c.compile(node.children[index], entry, passed)
			if err != nil {
				return 0, err
			}
		}
		return entry, nil
	case controlFirst:
		slot := c.checkpoints
		c.checkpoints = saturatingIncrement(c.checkpoints)
		next := passed
		for index := len(node.children) - 1; index >= 0; index-- {
			restore := c.emit(instruction{
				kind: instructionRestore,
				slot: slot,
				next: next,
			})
			var err error
			next, err = c.compile(node.children[index], handled, restore)
			if err != nil {
				return 0, err
			}
		}
		return c.emit(instruction{
			kind: instructionCheckpoint,
			slot: slot,
			next: next,
		}), nil
	default:
		return 0, errPrepareInvalid
	}
}

func (c *controlCompiler) emit(value instruction) int {
	index := len(c.instructions)
	c.instructions = append(c.instructions, value)
	return index
}

func executeCompiled[C any, M any, J Checkpoint[M]](
	prepared *PreparedControl,
	journal J,
	context *C,
	invoke func(LeafID, *C, J) (Outcome, error),
) (Outcome, error) {
	slots := make([]M, prepared.checkpointCount)
	set := make([]bool, prepared.checkpointCount)
	next := prepared.entry
	for {
		current := prepared.instructions[next]
		switch current.kind {
		case instructionInvoke:
			outcome, err := invoke(current.leaf, context, journal)
			if err != nil {
				return 0, err
			}
			switch checkedOutcome(outcome) {
			case OutcomeHandled:
				next = current.handled
			case OutcomePass:
				next = current.passed
			}
		case instructionCheckpoint:
			slots[current.slot] = journal.Checkpoint()
			set[current.slot] = true
			next = current.next
		case instructionRestore:
			if !set[current.slot] {
				panic("planning: compiled restore without checkpoint")
			}
			journal.Rollback(slots[current.slot])
			next = current.next
		case instructionHalt:
			return checkedOutcome(current.outcome), nil
		default:
			panic("planning: invalid control instruction")
		}
	}
}

func saturatingIncrement(value int) int {
	if value == int(^uint(0)>>1) {
		return value
	}
	return value + 1
}
