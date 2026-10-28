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
	pageID   int
	title    string
	active   bool
	activity []ActivityEntry
	scopes   []string
}

func (s *fakeSession) Page() (int, string, bool) {
	return s.pageID, s.title, s.active
}

func (s *fakeSession) Activity() []ActivityEntry {
	return append([]ActivityEntry(nil), s.activity...)
}

func (s *fakeSession) Scopes() []string {
	return append([]string(nil), s.scopes...)
}

func activeSession() *fakeSession {
	return &fakeSession{
		pageID: 1,
		title:  "Research notes",
		active: true,
		activity: []ActivityEntry{
			{Revision: 3, Characters: 8, Marks: 1},
			{Revision: 4, Characters: 9, Marks: 2},
		},
		scopes: []string{"Selection 2 → 4"},
	}
}
