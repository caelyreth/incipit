package storage

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// rescue is the last resort when the app quits with changes the notebook
// would not take: each such page's text goes to a plain file in the
// Recovery folder, readable with anything.
func (v *Saver) rescue() error {
	v.mu.Lock()
	pending := maps.Clone(v.pending)
	v.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	folder := filepath.Join(v.store.root, recoveryDir, "unsaved-"+v.now().Format(backupLayout))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("storage: create rescue folder: %w", err)
	}
	var failures error
	for key, page := range pending {
		name := filepath.Join(folder, fmt.Sprintf("page-%d.txt", key))
		if err := writeDurably(name, plainText(page.snapshot)); err != nil {
			failures = errors.Join(failures, fmt.Errorf("storage: rescue page %d: %w", key, err))
		}
	}
	return failures
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
	return writeDurablyWith(path, contents, defaultPublicationOps())
}

func writeDurablyWith(path, contents string, ops publicationOps) error {
	return writePreparedFileWith(path, func(target io.Writer) error {
		_, err := io.WriteString(target, contents)
		return err
	}, nil, ops)
}
