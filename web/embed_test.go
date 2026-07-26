package web

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/visionavtr/rustore-fdroid/internal"
)

func TestInstallAndWriteSigningInfo(t *testing.T) {
	dir := t.TempDir()
	idx := &internal.IndexV1{
		Repo: internal.Repo{
			Address:    "https://repo.example/apps",
			WebBaseURL: "https://repo.example/apps/packages",
		},
		Apps: []internal.App{{PackageName: "com.example.app", Name: "Example App"}},
	}
	if err := Install(dir, idx); err != nil {
		t.Fatalf("Install: %v", err)
	}
	page, err := os.ReadFile(filepath.Join(dir, "packages", "com.example.app", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "package=com.example.app") {
		t.Fatalf("app page does not link to package: %s", page)
	}

	fingerprint := strings.Repeat("AB", 32)
	if err := WriteSigningInfo(dir, idx, fingerprint); err != nil {
		t.Fatalf("WriteSigningInfo: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "repo-info.json"))
	if err != nil {
		t.Fatal(err)
	}
	var info RepoInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if info.Fingerprint != fingerprint || !strings.Contains(info.AddURL, "fingerprint="+fingerprint) {
		t.Fatalf("unexpected repo info: %+v", info)
	}
	if info.FDroidLink != "https://fdroid.link/#"+info.AddURL {
		t.Fatalf("unexpected F-Droid link: %q", info.FDroidLink)
	}
	qr, err := os.Open(filepath.Join(dir, "index.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer qr.Close()
	if _, err := png.DecodeConfig(qr); err != nil {
		t.Fatalf("index.png is not a valid PNG: %v", err)
	}

	if err := Remove(dir); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, name := range []string{"index.html", "index.png", "repo-info.json", generatedPackagesFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after Remove", name)
		}
	}
}
