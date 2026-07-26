package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/visionavtr/rustore-fdroid/internal"
)

func TestFinishPackageBatch_AllFailed(t *testing.T) {
	originalRepoPath := repoPath
	repoPath = t.TempDir()
	t.Cleanup(func() { repoPath = originalRepoPath })

	indexPath := internal.IndexV1Path(repoPath)
	originalIndex := []byte(`{"repo":{"version":7}}`)
	if err := os.WriteFile(indexPath, originalIndex, 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	err := finishPackageBatch(
		&internal.IndexV1{Repo: internal.Repo{Version: 7}},
		"update",
		2,
		0,
		[]error{errors.New("first failed"), errors.New("second failed")},
	)
	if err == nil {
		t.Fatal("finishPackageBatch returned nil after all packages failed")
	}
	if !strings.Contains(err.Error(), "failed to update all 2 packages") {
		t.Fatalf("unexpected error: %v", err)
	}

	currentIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if string(currentIndex) != string(originalIndex) {
		t.Fatalf("index changed after all packages failed: %s", currentIndex)
	}
}

func TestFinishPackageBatch_PartialSuccessSavesIndex(t *testing.T) {
	originalRepoPath := repoPath
	repoPath = t.TempDir()
	t.Cleanup(func() { repoPath = originalRepoPath })

	idx := &internal.IndexV1{Repo: internal.Repo{Version: 7}}
	if err := finishPackageBatch(idx, "update", 2, 1, []error{errors.New("one failed")}); err != nil {
		t.Fatalf("finishPackageBatch: %v", err)
	}

	saved, err := internal.LoadIndexV1(repoPath)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	if saved.Repo.Version != 8 {
		t.Fatalf("saved repo version = %d, want 8", saved.Repo.Version)
	}
}
