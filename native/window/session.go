package window

import (
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

type sheetSession struct {
	page       workspace.PageID
	controller *controller.DocumentController
	// key is the page's identity in the notebook.
	key storage.PageKey
	// saved is the revision last handed to the notebook; unsaved marks a
	// page never handed to it.
	saved   uint64
	unsaved bool
}

// noteSaved records that the page's current revision is with the notebook.
func (s *sheetSession) noteSaved() {
	s.saved = s.controller.Runtime().Revision()
	s.unsaved = false
}

func (s *sheetSession) Document() *controller.DocumentController { return s.controller }

// Title is the page's first heading.
func (s *sheetSession) Title() string { return s.controller.Session().Title() }

func (s *sheetSession) Configuration() kernel.PolicyConfiguration {
	return s.controller.Runtime().PolicyConfiguration()
}

func (s *sheetSession) AdoptConfiguration(configuration kernel.PolicyConfiguration) error {
	return s.controller.Adopt(configuration)
}

func (s *sheetSession) Page() (int, string, bool) {
	title := s.Title()
	if title == "" {
		title = workspace.DefaultTitle
	}
	return int(s.page), title, true
}

func (s *sheetSession) Activity() []settings.ActivityEntry {
	activity := s.controller.Activity()
	result := make([]settings.ActivityEntry, 0, len(activity))
	for _, item := range activity {
		result = append(result, settings.ActivityEntry{
			Revision:   item.Revision,
			Characters: item.Characters,
			Marks:      item.Marks,
		})
	}
	return result
}

func (s *sheetSession) Scopes() []string {
	out, ok, err := s.controller.Session().TabOffer(time.Now())
	if err != nil || !ok {
		return nil
	}
	return []string{out.Kind.String()}
}

type mygoServices struct{ services ui.Services }

func (s mygoServices) ReadClipboard() string      { return s.services.ReadClipboard() }
func (s mygoServices) WriteClipboard(text string) { s.services.WriteClipboard(text) }
