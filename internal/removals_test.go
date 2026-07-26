package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPendingRemovalsCommitAfterSave(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.apk")
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := &IndexV1{Packages: make(map[string][]Package)}
	if err := QueueFileRemoval(idx, "old.apk"); err != nil {
		t.Fatal(err)
	}
	if err := SaveIndexV1(dir, idx); err != nil {
		t.Fatalf("SaveIndexV1: %v", err)
	}
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatalf("old file removed before signing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, pendingRemovalsFile)); err != nil {
		t.Fatalf("pending removals manifest missing: %v", err)
	}

	loaded, err := LoadIndexV1(dir)
	if err != nil {
		t.Fatalf("LoadIndexV1: %v", err)
	}
	if len(loaded.pendingRemovals) != 1 || loaded.pendingRemovals[0] != "old.apk" {
		t.Fatalf("pending removals not restored: %+v", loaded.pendingRemovals)
	}
	if err := CommitPendingRemovals(dir); err != nil {
		t.Fatalf("CommitPendingRemovals: %v", err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("old file still exists after commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, pendingRemovalsFile)); !os.IsNotExist(err) {
		t.Fatalf("pending removals manifest still exists: %v", err)
	}
}

func TestQueueFileRemovalRejectsTraversal(t *testing.T) {
	idx := &IndexV1{}
	for _, name := range []string{"../outside", "/absolute", ""} {
		if err := QueueFileRemoval(idx, name); err == nil {
			t.Fatalf("QueueFileRemoval accepted %q", name)
		}
	}
}
