package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type IndexV1 struct {
	Repo     Repo                `json:"repo"`
	Requests Requests            `json:"requests"`
	Apps     []App               `json:"apps"`
	Packages map[string][]Package `json:"packages"`
}

type Repo struct {
	Timestamp   int64  `json:"timestamp"`
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

type Requests struct {
	Install   []any `json:"install"`
	Uninstall []any `json:"uninstall"`
}

type App struct {
	PackageName           string   `json:"packageName"`
	Added                 int64    `json:"added"`
	Icon                  string   `json:"icon"`
	License               string   `json:"license"`
	AntiFeatures          []string `json:"antiFeatures"`
	Name                  string   `json:"name"`
	Summary               string   `json:"summary"`
	Description           string   `json:"description"`
	AllowedAPKSigningKeys []string `json:"allowedAPKSigningKeys"`
	AuthorName            string   `json:"authorName"`
	Categories            []string `json:"categories"`
	SuggestedVersionName  string   `json:"suggestedVersionName"`
	SuggestedVersionCode  string   `json:"suggestedVersionCode"`
	LastUpdated           int64    `json:"lastUpdated"`
}

type Package struct {
	PackageName      string `json:"packageName"`
	Added            int64  `json:"added"`
	Size             int64  `json:"size"`
	APKName          string `json:"apkName"`
	HashType         string `json:"hashType"`
	Sig              string `json:"sig"`
	Signer           string `json:"signer"`
	MinSdkVersion    int    `json:"minSdkVersion"`
	TargetSdkVersion int    `json:"targetSdkVersion"`
	VersionCode      int    `json:"versionCode"`
	VersionName      string `json:"versionName"`
	Hash             string `json:"hash"`
}

func JavaTime() int64 {
	return time.Now().UnixMilli()
}

func TimestrToTimestamp(timestr string) (int64, error) {
	t, err := time.Parse(time.RFC3339, timestr)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", timestr)
		if err != nil {
			return 0, fmt.Errorf("parse time %q: %w", timestr, err)
		}
	}
	return t.UnixMilli(), nil
}

func IndexV1Path(repoPath string) string {
	return filepath.Join(repoPath, "index-v1.json")
}

func LoadIndexV1(repoPath string) (*IndexV1, error) {
	data, err := os.ReadFile(IndexV1Path(repoPath))
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	var idx IndexV1
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}

	if idx.Packages == nil {
		idx.Packages = make(map[string][]Package)
	}
	if idx.Apps == nil {
		idx.Apps = []App{}
	}

	return &idx, nil
}

func SaveIndexV1(repoPath string, idx *IndexV1) error {
	idx.Repo.Timestamp = JavaTime()
	idx.Repo.Version++

	data, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	if err := os.WriteFile(IndexV1Path(repoPath), data, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	return nil
}

func FindAppIndex(idx *IndexV1, packageName string) int {
	for i, app := range idx.Apps {
		if app.PackageName == packageName {
			return i
		}
	}
	return -1
}

func PackageContainsVersion(idx *IndexV1, packageName string, versionCode int) bool {
	for _, pkg := range idx.Packages[packageName] {
		if pkg.VersionCode == versionCode {
			return true
		}
	}
	return false
}
