package window

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/caelyreth/incipit/internal/app/storage"
)

// openingName is the file in the notebook's folder naming the page whose
// editor is opening, from when it is made until a frame with it is drawn
// whole or the app quits whole. Found at launch, that page's opening took the app down: it is
// left closed, its place forgotten, rather than opened in front again
// to take the app down at every launch.
const openingName = ".opening"

// markOpening notes that page key's editor is opening.
func (a *AppState) markOpening(key storage.PageKey) {
	if a.notebook == nil {
		return
	}
	path := filepath.Join(a.notebook.store.Root(), openingName)
	if err := os.WriteFile(path, []byte(strconv.FormatInt(int64(key), 10)), 0o644); err == nil {
		a.opening = true
	}
}

// openingDrawn lets the mark go once a frame was drawn whole after it.
func (a *AppState) openingDrawn() {
	if !a.opening || a.notebook == nil {
		return
	}
	a.opening = false
	_ = os.Remove(filepath.Join(a.notebook.store.Root(), openingName))
}

// lastOpening is the page whose opening the app last did not live
// through, read from root and let go of.
func lastOpening(root string) (storage.PageKey, bool) {
	path := filepath.Join(root, openingName)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	_ = os.Remove(path)
	key, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return storage.PageKey(key), err == nil
}

// withoutPage is layout with page key closed and its place forgotten.
func withoutPage(layout storage.Layout, key storage.PageKey) storage.Layout {
	front := -1
	if layout.HasFront && layout.Front >= 0 && layout.Front < len(layout.Columns) && layout.Columns[layout.Front].Page != key {
		front = layout.Front
	}
	var columns []storage.Column
	for index, col := range layout.Columns {
		if col.Page == key {
			continue
		}
		if index == front {
			front = len(columns)
		}
		columns = append(columns, col)
	}
	layout.Columns = columns
	layout.Front, layout.HasFront = max(front, 0), front >= 0 && len(columns) > 0
	layout.Scroll = without(layout.Scroll, key)
	layout.Views = without(layout.Views, key)
	return layout
}

// without is m without key, m itself untouched.
func without[V any](m map[storage.PageKey]V, key storage.PageKey) map[storage.PageKey]V {
	out := make(map[storage.PageKey]V, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}
