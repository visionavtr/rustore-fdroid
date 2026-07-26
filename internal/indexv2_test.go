package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildIndexV2(t *testing.T) {
	dir := t.TempDir()
	iconData := []byte("icon")
	screenshotData := []byte("screenshot")
	if err := os.MkdirAll(filepath.Join(dir, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "icons", "app.png"), iconData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("frontend"), 0o644); err != nil {
		t.Fatal(err)
	}
	screenshotName := "com.example.app/ru/phoneScreenshots/01.png"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, screenshotName)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, screenshotName), screenshotData, 0o644); err != nil {
		t.Fatal(err)
	}
	apkData := []byte("apk")
	if err := os.WriteFile(filepath.Join(dir, "com.example.app_42.apk"), apkData, 0o644); err != nil {
		t.Fatal(err)
	}
	screenshotHash := sha256.Sum256(screenshotData)
	apkHash := strings.Repeat("a", sha256.Size*2)

	idx := &IndexV1{
		Repo: Repo{
			Timestamp:   123456789,
			Name:        "Test Repo",
			Address:     "https://example.com/repo",
			Description: "Test repository",
		},
		Apps: []App{{
			PackageName:      "com.example.app",
			Added:            100,
			LastUpdated:      200,
			Icon:             "app.png",
			Name:             "Example",
			Summary:          "Summary",
			Description:      "Description",
			AuthorName:       "Developer",
			AuthorEmail:      "dev@example.com",
			AuthorWebSite:    "https://example.com",
			Categories:       []string{"tools"},
			AntiFeatures:     []string{"NoSourceSince"},
			PhoneScreenshots: []RepoFile{{Name: screenshotName, SHA256: hex.EncodeToString(screenshotHash[:]), Size: int64(len(screenshotData))}},
		}},
		Packages: map[string][]Package{
			"com.example.app": {{
				PackageName:         "com.example.app",
				Added:               200,
				Size:                int64(len(apkData)),
				APKName:             "com.example.app_42.apk",
				Hash:                apkHash,
				VersionCode:         42,
				VersionName:         "4.2",
				MinSdkVersion:       23,
				TargetSdkVersion:    35,
				MaxSdkVersion:       36,
				Signers:             []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
				NativeCode:          []string{"x86_64", "arm64-v8a"},
				Features:            []string{"android.hardware.camera"},
				UsesPermission:      [][]any{{"android.permission.CAMERA", nil}, {"android.permission.READ_MEDIA_IMAGES", int32(34)}},
				UsesPermissionSDK23: [][]any{{"android.permission.POST_NOTIFICATIONS", nil}},
				WhatsNew:            "New release",
			}},
		},
	}

	data, err := BuildIndexV2(dir, idx)
	if err != nil {
		t.Fatalf("BuildIndexV2: %v", err)
	}
	var result IndexV2
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse index-v2: %v", err)
	}
	if result.Repo.WebBaseURL != "https://example.com/repo/packages" {
		t.Fatalf("webBaseUrl = %q", result.Repo.WebBaseURL)
	}
	app := result.Packages["com.example.app"]
	if app.Metadata.Name[AppLocale] != "Example" || app.Metadata.AuthorEmail != "dev@example.com" {
		t.Fatalf("unexpected metadata: %+v", app.Metadata)
	}
	if app.Metadata.Icon[AppLocale].Name != "/icons/app.png" {
		t.Fatalf("unexpected icon: %+v", app.Metadata.Icon)
	}
	if app.Metadata.Screenshots.Phone[AppLocale][0].Name != "/"+screenshotName {
		t.Fatalf("unexpected screenshots: %+v", app.Metadata.Screenshots)
	}
	version, ok := app.Versions[apkHash]
	if !ok {
		t.Fatalf("version key %s missing", apkHash)
	}
	if version.Manifest.VersionCode != 42 || version.Manifest.MaxSdkVersion != 36 {
		t.Fatalf("unexpected manifest: %+v", version.Manifest)
	}
	if version.Manifest.UsesSDK.TargetSdkVersion != 35 || len(version.Manifest.NativeCode) != 2 {
		t.Fatalf("unexpected SDK/native code: %+v", version.Manifest)
	}
	if version.Manifest.UsesPermission[1].MaxSdkVersion == nil || *version.Manifest.UsesPermission[1].MaxSdkVersion != 34 {
		t.Fatalf("permission max SDK missing: %+v", version.Manifest.UsesPermission)
	}
	if version.WhatsNew[AppLocale] != "New release" || len(version.AntiFeatures) != 1 {
		t.Fatalf("unexpected version metadata: %+v", version)
	}

	entryData, err := BuildEntry(idx, data, IndexV2PackageCount(idx))
	if err != nil {
		t.Fatalf("BuildEntry: %v", err)
	}
	var entry Entry
	if err := json.Unmarshal(entryData, &entry); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if entry.Index.SHA256 != hex.EncodeToString(hash[:]) || entry.Index.Size != int64(len(data)) {
		t.Fatalf("entry does not describe index-v2: %+v", entry.Index)
	}
	if entry.Version != MetadataVersion || entry.MaxAge != DefaultMaxAge || entry.Index.NumPackages != 1 || entry.Diffs == nil {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestBuildIndexV2RejectsMissingAPK(t *testing.T) {
	idx := &IndexV1{
		Apps: []App{{PackageName: "com.example.app"}},
		Packages: map[string][]Package{
			"com.example.app": {{APKName: "missing.apk", Hash: strings.Repeat("a", 64)}},
		},
	}
	if _, err := BuildIndexV2(t.TempDir(), idx); err == nil {
		t.Fatal("BuildIndexV2 accepted a missing APK")
	}
}

func TestBuildIndexV2OmitsWebBaseWithoutFrontend(t *testing.T) {
	idx := &IndexV1{Repo: Repo{Address: "https://example.com/repo"}}
	data, err := BuildIndexV2(t.TempDir(), idx)
	if err != nil {
		t.Fatalf("BuildIndexV2: %v", err)
	}
	var result IndexV2
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Repo.WebBaseURL != "" {
		t.Fatalf("webBaseUrl = %q without a frontend", result.Repo.WebBaseURL)
	}
}
