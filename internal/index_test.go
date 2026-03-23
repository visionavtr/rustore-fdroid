package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveIndexV1_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &IndexV1{
		Repo: Repo{
			Name:        "Test Repo",
			Address:     "https://example.com/repo",
			Description: "test",
			Version:     1,
		},
		Apps: []App{
			{
				PackageName: "ru.sberbankmobile",
				Name:        "Sberbank",
				Summary:     "Mobile banking",
			},
			{
				PackageName: "ru.autodoc.autodocapp",
				Name:        "Autodoc",
				Summary:     "Auto parts",
			},
		},
		Packages: map[string][]Package{
			"ru.sberbankmobile": {
				{
					PackageName: "ru.sberbankmobile",
					VersionCode: 100,
					VersionName: "1.0.0",
					APKName:     "ru.sberbankmobile_100.apk",
					HashType:    "sha256",
					Hash:        "abc123",
				},
			},
			"ru.autodoc.autodocapp": {
				{
					PackageName: "ru.autodoc.autodocapp",
					VersionCode: 50,
					VersionName: "2.1.0",
					APKName:     "ru.autodoc.autodocapp_50.apk",
					HashType:    "sha256",
					Hash:        "def456",
				},
			},
		},
	}

	if err := SaveIndexV1(dir, original); err != nil {
		t.Fatalf("SaveIndexV1: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "index-v1.json")); err != nil {
		t.Fatalf("index file not created: %v", err)
	}

	loaded, err := LoadIndexV1(dir)
	if err != nil {
		t.Fatalf("LoadIndexV1: %v", err)
	}

	if len(loaded.Apps) != 2 {
		t.Errorf("got %d apps, want 2", len(loaded.Apps))
	}
	if loaded.Apps[0].PackageName != "ru.sberbankmobile" {
		t.Errorf("first app = %q, want ru.sberbankmobile", loaded.Apps[0].PackageName)
	}
	if loaded.Apps[1].PackageName != "ru.autodoc.autodocapp" {
		t.Errorf("second app = %q, want ru.autodoc.autodocapp", loaded.Apps[1].PackageName)
	}

	sberPkgs := loaded.Packages["ru.sberbankmobile"]
	if len(sberPkgs) != 1 || sberPkgs[0].VersionCode != 100 {
		t.Errorf("sberbank packages unexpected: %+v", sberPkgs)
	}

	autodocPkgs := loaded.Packages["ru.autodoc.autodocapp"]
	if len(autodocPkgs) != 1 || autodocPkgs[0].VersionCode != 50 {
		t.Errorf("autodoc packages unexpected: %+v", autodocPkgs)
	}
}

func TestLoadIndexV1_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadIndexV1(dir)
	if err == nil {
		t.Fatal("expected error for missing index, got nil")
	}
}

func TestLoadIndexV1_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index-v1.json"), []byte("not json"), 0o644)
	_, err := LoadIndexV1(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFindAppIndex(t *testing.T) {
	idx := &IndexV1{
		Apps: []App{
			{PackageName: "ru.sberbankmobile"},
			{PackageName: "ru.autodoc.autodocapp"},
		},
	}

	if got := FindAppIndex(idx, "ru.sberbankmobile"); got != 0 {
		t.Errorf("FindAppIndex(sberbank) = %d, want 0", got)
	}
	if got := FindAppIndex(idx, "ru.autodoc.autodocapp"); got != 1 {
		t.Errorf("FindAppIndex(autodoc) = %d, want 1", got)
	}
	if got := FindAppIndex(idx, "com.nonexistent"); got != -1 {
		t.Errorf("FindAppIndex(nonexistent) = %d, want -1", got)
	}
}

func TestPackageContainsVersion(t *testing.T) {
	idx := &IndexV1{
		Packages: map[string][]Package{
			"ru.sberbankmobile": {
				{VersionCode: 100},
				{VersionCode: 101},
			},
		},
	}

	if !PackageContainsVersion(idx, "ru.sberbankmobile", 100) {
		t.Error("expected version 100 to exist")
	}
	if PackageContainsVersion(idx, "ru.sberbankmobile", 999) {
		t.Error("version 999 should not exist")
	}
	if PackageContainsVersion(idx, "com.nonexistent", 1) {
		t.Error("nonexistent package should not have versions")
	}
}

func TestTimestrToTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2024-01-15T10:30:00Z", false},
		{"no timezone", "2024-01-15T10:30:00", false},
		{"empty returns now", "", false},
		{"invalid format", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := TimestrToTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TimestrToTimestamp(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && ts <= 0 {
				t.Errorf("TimestrToTimestamp(%q) = %d, want > 0", tt.input, ts)
			}
		})
	}
}
