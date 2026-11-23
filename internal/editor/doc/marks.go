package doc

import (
	"slices"
	"strings"
	"sync"
)

// Mark is a mark kind with its attributes, such as a link's target. A mark
// has no identity: two equal marks are the same mark.
type Mark struct {
	Kind  MarkKind
	Attrs Attrs
}

// MarkSet is an interned, immutable set of marks. Two runs carry the same
// marks exactly when they hold the same pointer. The empty set is a
// pointer too, never nil, once interned.
type MarkSet struct {
	marks []Mark
	key   string
	in    *Interner
}

// Marks returns the set's marks in kind order. The slice must not change.
func (s *MarkSet) Marks() []Mark {
	if s == nil {
		return nil
	}
	return s.marks
}

func (s *MarkSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.marks)
}

// Empty reports whether the set holds no marks.
func (s *MarkSet) Empty() bool { return s.Len() == 0 }

// Has reports whether the set holds a mark of kind k.
func (s *MarkSet) Has(k MarkKind) bool {
	_, ok := s.Get(k)
	return ok
}

// Get returns the set's mark of kind k.
func (s *MarkSet) Get(k MarkKind) (Mark, bool) {
	for _, m := range s.Marks() {
		if m.Kind == k {
			return m, true
		}
	}
	return Mark{}, false
}

// With returns the set with m added, replacing a mark of the same kind.
// Applying a mark already present changes nothing.
func (s *MarkSet) With(m Mark) *MarkSet {
	marks := make([]Mark, 0, s.Len()+1)
	for _, old := range s.Marks() {
		if old.Kind != m.Kind {
			marks = append(marks, old)
		}
	}
	return s.in.Intern(append(marks, m))
}

// Without returns the set with the mark of kind k removed.
func (s *MarkSet) Without(k MarkKind) *MarkSet {
	if !s.Has(k) {
		return s
	}
	marks := make([]Mark, 0, s.Len())
	for _, old := range s.Marks() {
		if old.Kind != k {
			marks = append(marks, old)
		}
	}
	return s.in.Intern(marks)
}

// Interner is a page's append-only table of mark sets.
type Interner struct {
	mu    sync.Mutex
	sets  map[string]*MarkSet
	empty *MarkSet
}

// NewInterner returns an empty intern table.
func NewInterner() *Interner {
	in := &Interner{sets: map[string]*MarkSet{}}
	in.empty = &MarkSet{key: "", in: in}
	in.sets[""] = in.empty
	return in
}

// Empty is the empty set.
func (in *Interner) Empty() *MarkSet { return in.empty }

// Intern returns the one set holding marks. A later mark of a kind
// replaces an earlier one.
func (in *Interner) Intern(marks []Mark) *MarkSet {
	if len(marks) == 0 {
		return in.empty
	}
	sorted := make([]Mark, 0, len(marks))
	for _, m := range marks {
		i, found := slices.BinarySearchFunc(sorted, m.Kind, func(a Mark, k MarkKind) int { return int(a.Kind) - int(k) })
		if found {
			sorted[i] = m
			continue
		}
		sorted = slices.Insert(sorted, i, m)
	}
	var b strings.Builder
	for _, m := range sorted {
		b.WriteString(itoa(int(m.Kind)))
		b.WriteByte('[')
		m.Attrs.key(&b)
		b.WriteByte(']')
	}
	key := b.String()
	in.mu.Lock()
	defer in.mu.Unlock()
	if set, ok := in.sets[key]; ok {
		return set
	}
	set := &MarkSet{marks: sorted, key: key, in: in}
	in.sets[key] = set
	return set
}

// Owns reports whether s was interned here.
func (in *Interner) Owns(s *MarkSet) bool { return s != nil && s.in == in }

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
