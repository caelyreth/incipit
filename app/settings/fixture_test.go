package settings

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

func editorCatalog(t *testing.T) kernel.PolicyCatalog {
	t.Helper()
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func editorHost(t *testing.T, catalog kernel.PolicyCatalog) *HostStore {
	t.Helper()
	host, err := NewHostStore(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return host
}

type fakeSession struct {
	pageID int
	title  string
	active bool
}

func (s *fakeSession) Page() (int, string, bool) {
	return s.pageID, s.title, s.active
}

func activeSession() *fakeSession {
	return &fakeSession{
		pageID: 1,
		title:  "Research notes",
		active: true,
	}
}
