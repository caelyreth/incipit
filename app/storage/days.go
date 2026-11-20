package storage

import (
	"context"
	"database/sql"
	"time"
)

// DayOf names the day at falls on, where it falls, as the notebook keeps
// the words written each day ("2006-01-02").
func DayOf(at time.Time) string { return at.Format(time.DateOnly) }

// loadWritten reads the words written each day, by DayOf.
func loadWritten(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT day, words FROM days`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	written := map[string]int{}
	for rows.Next() {
		var day string
		var words int
		if err := rows.Scan(&day, &words); err != nil {
			return nil, err
		}
		written[day] = words
	}
	return written, rows.Err()
}

// writeWritten keeps the words written on each day given.
func writeWritten(ctx context.Context, tx *sql.Tx, written map[string]int) error {
	for day, words := range written {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO days (day, words) VALUES (?, ?)
			ON CONFLICT (day) DO UPDATE SET words = excluded.words`, day, words); err != nil {
			return err
		}
	}
	return nil
}
