package storage

import (
	"context"
	"database/sql"
)

// settingsKey is the layout table's key for the app's settings record. It
// sits beside the workspace layout instead of in a schema of its own: the
// notebook already has a durable key-value row for a small record, and a
// notebook without it simply starts from defaults.
const settingsKey = "settings"

// LoadSettings returns the settings record, or nil for a notebook that
// has never saved one.
func (s *Store) LoadSettings() ([]byte, error) {
	var encoded string
	err := s.db.QueryRowContext(context.Background(),
		`SELECT value FROM layout WHERE key = ?`, settingsKey).Scan(&encoded)
	switch err {
	case nil:
		return []byte(encoded), nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}
