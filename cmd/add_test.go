package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSyncPhoneScreenshotsSortsAndReusesFiles(t *testing.T) {
	originalRepoPath := repoPath
	repoPath = t.TempDir()
	t.Cleanup(func() { repoPath = originalRepoPath })

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer server.Close()

	info := &internal.AppInfo{
		PackageName: "com.example.app",
		FileURLs: []internal.AppFile{
			{URL: server.URL + "/second.jpg?token=x", Ordinal: 2, Type: "SCREENSHOT"},
			{URL: server.URL + "/first.png?token=x", Ordinal: 1, Type: "SCREENSHOT"},
			{URL: server.URL + "/ignored.png", Ordinal: 0, Type: "BANNER"},
		},
	}
	idx := &internal.IndexV1{}
	files, err := syncPhoneScreenshots(idx, info, nil)
	if err != nil {
		t.Fatalf("syncPhoneScreenshots: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("screenshots = %d, want 2", len(files))
	}
	if !strings.HasSuffix(files[0].Name, "/01.png") || !strings.HasSuffix(files[1].Name, "/02.jpg") {
		t.Fatalf("screenshots are not sorted/named correctly: %+v", files)
	}
	for _, file := range files {
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(file.Name))); err != nil {
			t.Fatalf("screenshot %s missing: %v", file.Name, err)
		}
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}

	reused, err := syncPhoneScreenshots(idx, info, files)
	if err != nil {
		t.Fatalf("syncPhoneScreenshots reuse: %v", err)
	}
	if requests != 2 {
		t.Fatalf("unchanged screenshots were downloaded again: %d requests", requests)
	}
	if len(reused) != len(files) || reused[0].SHA256 != files[0].SHA256 {
		t.Fatalf("reused screenshots changed: %+v", reused)
	}
}

func TestIconExtFromURLIgnoresQuery(t *testing.T) {
	if got := iconExtFromURL("https://example.com/icon.WEBP?size=512"); got != ".webp" {
		t.Fatalf("iconExtFromURL = %q, want .webp", got)
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
	if saved.Repo.Version != internal.MetadataVersion {
		t.Fatalf("saved repo version = %d, want %d", saved.Repo.Version, internal.MetadataVersion)
	}
}
