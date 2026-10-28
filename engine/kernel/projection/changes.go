package projection

import "github.com/caelyreth/neverland/engine/model"

// Dependency is a projection-affecting document or resource state key.
type Dependency struct {
	Document bool
	Resource string
}

// Changes is the projection-facing subset of an engine publication.
type Changes struct {
	Document  []model.Change
	TextEdits []model.TextEdit
	Resources []string
}
