package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// rescue is the last resort when the app quits with changes the notebook
// would not take: each such page's text goes to a plain file in the
// Recovery folder, readable with anything.
func (v *Saver) rescue() {
	v.mu.Lock()
	pending := v.pending
	v.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	folder := filepath.Join(v.store.root, recoveryDir, "unsaved-"+v.now().Format(backupLayout))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return
	}
	for key, page := range pending {
		name := filepath.Join(folder, fmt.Sprintf("page-%d.txt", key))
		_ = writeDurably(name, plainText(page.snapshot))
	}
	_ = syncDir(folder)
}

// plainText is a page's text, a line per block.
func plainText(snapshot model.DocumentSnapshot) string {
	root, ok := snapshot.Node(snapshot.Root())
	if !ok {
		return ""
	}
	lines := make([]string, 0, len(root.Children()))
	for _, id := range root.Children() {
		if node, ok := snapshot.Node(id); ok {
			if text, ok := node.TextRoot(); ok {
				lines = append(lines, text.String())
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func writeDurably(path, contents string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
