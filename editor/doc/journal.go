package doc

import "time"

// OriginClass is the class of intent a change came from.
type OriginClass uint8

const (
	// OriginTyping is text the writer entered, by key, input method or
	// dictation alike, and the keys that edit it.
	OriginTyping OriginClass = iota + 1
	OriginPaste
	OriginCommand
	OriginPointer
	OriginAgent
	OriginHistory
	OriginTimer
)

// Phase is the stage of a transaction that produced a change: the
// handler's, or a later one the editing layer numbers after it. Each
// phase's steps are a history layer of their own.
type Phase uint8

// PhaseHandler is the stage of the intent's own handler.
const PhaseHandler Phase = 0

// Origin says what produced a record: an intent class and a phase. It
// never names a policy.
type Origin struct {
	Class OriginClass
	Phase Phase
}

// RecordKind is the kind of a journal record.
type RecordKind uint8

const (
	RecordTextInserted RecordKind = iota + 1
	RecordTextDeleted
	RecordMarks
	RecordNodeChanged
	RecordNodesInserted
	RecordNodesRemoved
	RecordNodeMoved
	RecordTransferred
	RecordSelection
	RecordView
)

// Record is one entry of a transaction's journal, with the semantic detail
// rules and observers test.
type Record struct {
	Kind   RecordKind
	Origin Origin
	// Policy names the policy that produced the record, for traces only.
	Policy string
	// Slot and From, To locate text inserted, deleted or remarked; for an
	// insertion From, To is the inserted range after the change, for a
	// deletion the empty range left.
	Slot     SlotRef
	From, To int
	// Text is the text inserted or deleted.
	Text string
	// Marks are the mark kinds a Mark changed.
	Marks []MarkKind
	// Node, OldKind and NewKind describe a node changed.
	Node             ID
	OldKind, NewKind Kind
	// Nodes are the subtrees inserted, removed or moved.
	Nodes []ID
	// Target and At are where a transfer or a move went.
	Target SlotRef
	At     int
	// View names the view a selection or view record is about.
	View uint64
}

// Mask is a bitset of record kinds, origin classes and phases, so a rule's
// trigger is one AND.
type Mask uint64

const (
	maskClassShift = 16
	maskPhaseShift = 32
)

// KindBit is the bit of a record kind.
func KindBit(k RecordKind) Mask { return 1 << k }

// ClassBit is the bit of an origin class.
func ClassBit(c OriginClass) Mask { return 1 << (maskClassShift + c) }

// PhaseBit is the bit of a phase.
func PhaseBit(p Phase) Mask { return 1 << (maskPhaseShift + p) }

// Bits is the mask of one record.
func (r Record) Bits() Mask {
	return KindBit(r.Kind) | ClassBit(r.Origin.Class) | PhaseBit(r.Origin.Phase)
}

// Journal is a transaction's ordered records.
type Journal struct {
	Records []Record
	mask    Mask
}

// Add appends a record.
func (j *Journal) Add(r Record) {
	j.Records = append(j.Records, r)
	j.mask |= r.Bits()
}

// Mask is the union of the records' bits.
func (j *Journal) Mask() Mask { return j.mask }

// Since returns the records from index i on.
func (j *Journal) Since(i int) []Record { return j.Records[i:] }

// Stamp is when an intent happened, as the host said. The document never
// reads a clock.
type Stamp = time.Time
