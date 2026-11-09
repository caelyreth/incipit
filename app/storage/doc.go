// Package storage keeps the notebook on disk, so that what is written
// survives a quit, a crash or a lost machine state.
//
// The notebook is one SQLite database in a Neverland folder of the user's
// Documents (Root). Memory stays the source of truth while the app runs: a
// Saver takes what changed and writes it in one transaction, off the UI
// thread. What it relies on, from the bottom up:
//
//   - Every write is a transaction in a write-ahead log, synced with a full
//     flush to the disk's platter (F_FULLFSYNC on macOS) before it counts.
//     A crash or power loss leaves the last committed state; the log is
//     replayed when the notebook next opens.
//   - The Saver writes a quarter second after changes pause, and within a
//     second while they go on: the most a crash can take back. Quitting
//     writes what is pending before the app goes.
//   - On macOS the folder carries a .nosync marker and the File Provider's
//     ignore attribute, so iCloud's Desktop & Documents sync never uploads
//     the database mid-write or evicts it. Syncing on purpose is for native
//     code to do later.
//   - One process holds the notebook at a time (a lock file beside it), so
//     two windows never write past each other.
//   - Opening checks the database. A damaged one is moved aside, never
//     deleted, and the newest backup that checks out takes its place.
//   - Backups are verified copies made while the app runs and when it
//     quits, kept newest first and then one a day.
package storage
