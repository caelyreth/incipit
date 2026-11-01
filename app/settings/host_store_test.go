package settings

import (
	"errors"
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

func TestHostStoreDoesNotPublishFailedCandidate(t *testing.T) {
	failed := errors.New("publication failed")
	var offered kernel.PolicyConfiguration
	store, err := NewHostStoreWithPublisher(editorCatalog(t), func(configuration kernel.PolicyConfiguration) error {
		offered = configuration
		return failed
	})
	if err != nil {
		t.Fatal(err)
	}
	before := store.Snapshot()
	candidate := before.Configuration.Clone()
	candidate.SetEnabled(editor.EditorPolicyPairs.ID(), false)
	if err := store.Adopt(candidate); !errors.Is(err, failed) {
		t.Fatalf("adopt error = %v, want publication failure", err)
	}
	if offered.IsEnabled(editor.EditorPolicyPairs.ID()) {
		t.Fatal("publisher did not receive the candidate")
	}
	after := store.Snapshot()
	if !after.Configuration.Equal(before.Configuration) ||
		after.Prepared.IsEnabled(editor.EditorPolicyPairs.ID()) != before.Prepared.IsEnabled(editor.EditorPolicyPairs.ID()) {
		t.Fatal("failed publication changed the store")
	}
}

func TestHostStoreSnapshotsStayCoherentAndOwned(t *testing.T) {
	store := editorHost(t, editorCatalog(t))
	policy := editor.EditorPolicyPairs.ID()
	old := store.Snapshot()
	old.Configuration.SetEnabled(policy, false)
	current := store.Configuration()
	if !current.IsEnabled(policy) {
		t.Fatal("changing a snapshot changed the store")
	}
	errors := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			snapshot := store.Snapshot()
			if snapshot.Configuration.IsEnabled(policy) != snapshot.Prepared.IsEnabled(policy) {
				errors <- fmt.Errorf("snapshot combined different policy generations")
				return
			}
		}
	}()
	for index := range 200 {
		candidate := store.Configuration()
		candidate.SetEnabled(policy, index%2 == 0)
		if err := store.Adopt(candidate); err != nil {
			t.Fatal(err)
		}
	}
	<-done
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
}

func TestHostStoreValidatesBeforePublishingAndOwnsPublishedValues(t *testing.T) {
	var published kernel.PolicyConfiguration
	calls := 0
	store, err := NewHostStoreWithPublisher(editorCatalog(t), func(configuration kernel.PolicyConfiguration) error {
		calls++
		published = configuration
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := store.Configuration()
	invalid.SetSetting(kernel.NewSettingKey("missing.policy", "x"), kernel.BoolSetting(true))
	if err := store.Adopt(invalid); err == nil || calls != 0 {
		t.Fatalf("invalid candidate: error = %v, publisher calls = %d", err, calls)
	}
	candidate := store.Configuration()
	candidate.SetEnabled(editor.EditorPolicyPairs.ID(), false)
	if err := store.Adopt(candidate); err != nil || calls != 1 {
		t.Fatalf("valid candidate: error = %v, publisher calls = %d", err, calls)
	}
	published.SetEnabled(editor.EditorPolicyPairs.ID(), true)
	candidate.SetEnabled(editor.EditorPolicyPairs.ID(), true)
	current := store.Configuration()
	if current.IsEnabled(editor.EditorPolicyPairs.ID()) {
		t.Fatal("publisher or caller changed the installed configuration")
	}
}
