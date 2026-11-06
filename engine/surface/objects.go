package surface

import (
	"fmt"
	"slices"
	"sort"

	"github.com/caelyreth/neverland/engine/model"
)

// Object is what a run of a block's text stands for, set in its place
// rather than as its characters: a checkbox for a list's marker, a
// picture for its line's object, math for a formula. A flow's objects are
// one ordered list, whatever their kind; a view asks which stand where.
type Object interface {
	// Run is the run of text the object stands for.
	Run() model.UTF16Range
	// node is the block the object is the notation of.
	node() model.NodeID
	// validate checks the object against the flow and the block it is in.
	validate(f *Flow, block Block) error
	// shifted is the object after an edit of the flow, or false when the
	// edit takes it away.
	shifted(edit rangeEdit) (Object, bool)
	// moved is the object by units further on, its text and all.
	moved(by model.UTF16Offset) Object
}

// by is r by units further on.
func by(r model.UTF16Range, units model.UTF16Offset) model.UTF16Range {
	return model.NewUTF16Range(r.Start+units, r.End+units)
}

// WithObjects is the flow with objects standing for its text, in order,
// apart, each within one block.
func (f *Flow) WithObjects(objects []Object) (*Flow, error) {
	previous := model.UTF16Offset(-1)
	for _, object := range objects {
		if err := f.admits(object, previous); err != nil {
			return nil, err
		}
		previous = object.Run().End
	}
	next := *f
	next.objects = slices.Clone(objects)
	return &next, nil
}

// admits checks object can stand in the flow after one ending at previous.
func (f *Flow) admits(object Object, previous model.UTF16Offset) error {
	span := object.Run()
	if span.Empty() || span.Start < previous || span.End > f.UTF16Len() {
		return fmt.Errorf("objects must be in order, apart")
	}
	block, _, ok := f.BlockAt(span.Start)
	if !ok || span.End > block.Range.End || object.node() != block.Node {
		return fmt.Errorf("an object must stand for notation of its own block")
	}
	return object.validate(f, block)
}

// Objects are the flow's objects, in order.
func (f *Flow) Objects() []Object { return slices.Clone(f.objects) }

// ObjectsIn are the objects overlapping range16, in order; an empty range
// finds those holding its offset.
func (f *Flow) ObjectsIn(range16 model.UTF16Range) []Object {
	index := sort.Search(len(f.objects), func(index int) bool {
		return f.objects[index].Run().End > range16.Start
	})
	end := index
	for end < len(f.objects) && (f.objects[end].Run().Start < range16.End ||
		range16.Empty() && f.objects[end].Run().Start <= range16.Start) {
		end++
	}
	return slices.Clone(f.objects[index:end])
}

// objectAt is the object whose run is run.
func (f *Flow) objectAt(run model.UTF16Range) (Object, bool) {
	for _, object := range f.ObjectsIn(run) {
		if object.Run() == run {
			return object, true
		}
	}
	return nil, false
}

// ObjectsOf are the flow's objects of one kind, in order.
func ObjectsOf[T Object](f *Flow) []T {
	var found []T
	for _, object := range f.objects {
		if typed, ok := object.(T); ok {
			found = append(found, typed)
		}
	}
	return found
}

// ObjectOf is the object of one kind standing for the text at offset.
func ObjectOf[T Object](f *Flow, offset model.UTF16Offset) (T, bool) {
	for _, object := range f.ObjectsIn(model.EmptyUTF16Range(offset)) {
		if typed, ok := object.(T); ok && object.Run().Start <= offset && offset < object.Run().End {
			return typed, true
		}
	}
	var none T
	return none, false
}
