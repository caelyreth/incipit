// Package projection maps semantic text to versioned editable visual text.
package projection

// Version identifies a source revision together with a projection generation.
// The generation changes when presentation is rebuilt without semantic text
// changing.
type Version struct {
	sourceRevision uint64
	generation     uint64
}

// ProjectionVersion is an explicit alias used by engine and editor packages.
type ProjectionVersion = Version

func NewVersion(sourceRevision, generation uint64) Version {
	return Version{sourceRevision: sourceRevision, generation: generation}
}

func (v Version) SourceRevision() uint64 { return v.sourceRevision }

func (v Version) Generation() uint64 { return v.generation }
