package kernel

import (
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func ProjectionDependencies(keys []StateKey) []projection.Dependency {
	dependencies := make([]projection.Dependency, 0, len(keys))
	for _, key := range keys {
		dependencies = append(dependencies, projection.Dependency{
			Document: key.Document,
			Resource: string(key.Resource),
		})
	}
	return dependencies
}

func (c EngineChanges) ProjectionChanges() projection.Changes {
	resources := make([]string, 0, len(c.resources))
	for _, change := range c.resources {
		resources = append(resources, string(change.ID))
	}
	return projection.Changes{
		Document:  append([]model.Change(nil), c.document...),
		TextEdits: append([]model.TextEdit(nil), c.textEdits...),
		Resources: resources,
	}
}
