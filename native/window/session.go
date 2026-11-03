package window

import (
	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/transfer"
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
	// wordCount is the page's words at revision counted.
	wordCount int
	counted   uint64
	hasCount  bool
}

// words is how many words the page holds, counted again only once it
// changed.
func (s *sheetSession) words() int {
	revision := s.controller.Runtime().Revision()
	if !s.hasCount || s.counted != revision {
		metrics, ok := s.controller.MetricsForCurrentText()
		if !ok {
			metrics = s.controller.Metrics()
		}
		s.wordCount, s.counted, s.hasCount = metrics.Words, revision, true
	}
	return s.wordCount
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

type mygoServices struct{ services ui.Services }

func (s mygoServices) ReadClipboard() string      { return s.services.ReadClipboard() }
func (s mygoServices) WriteClipboard(text string) { s.services.WriteClipboard(text) }

func (s mygoServices) ClipboardFormats() []transfer.Format { return s.services.ClipboardFormats() }

func (s mygoServices) ReadClipboardData(formats ...transfer.Format) transfer.Data {
	return s.services.ReadClipboardData(formats...)
}

func (s mygoServices) WriteClipboardData(data transfer.Data) { s.services.WriteClipboardData(data) }
