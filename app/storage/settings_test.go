package storage

import (
	"bytes"
	"testing"
)

func TestSettingsLiveBesideTheLayoutRecord(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	if data, err := store.LoadSettings(); err != nil || data != nil {
		t.Fatalf("new notebook settings = %q, %v", data, err)
	}
	saver := store.Autosave()
	record := []byte(`{"version":1,"zen":true}`)
	saver.Settings(record)
	record[0] = 'x'
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(`{"version":1,"zen":true}`)) {
		t.Fatalf("settings = %q", got)
	}
	// Settings do not require a layout row to share the table.
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if notebook.HasLayout {
		t.Fatal("settings alone were read as a workspace layout")
	}
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
