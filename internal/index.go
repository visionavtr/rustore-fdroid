package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	MetadataVersion = 30000
	DefaultMaxAge   = 14
	RepoLocale      = "en-US"
	AppLocale       = "ru"
)

type IndexV1 struct {
	Repo            Repo                 `json:"repo"`
	Requests        Requests             `json:"requests"`
	Apps            []App                `json:"apps"`
	Packages        map[string][]Package `json:"packages"`
	pendingRemovals []string
}

type Repo struct {
	Timestamp   int64  `json:"timestamp"`
	Version     int    `json:"version"`
	MaxAge      int    `json:"maxage,omitempty"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Address     string `json:"address"`
	WebBaseURL  string `json:"webBaseUrl,omitempty"`
	Description string `json:"description"`
}

type Requests struct {
	Install   []any `json:"install"`
	Uninstall []any `json:"uninstall"`
}

type App struct {
	PackageName           string                  `json:"packageName"`
	Added                 int64                   `json:"added"`
	Icon                  string                  `json:"icon"`
	License               string                  `json:"license"`
	AntiFeatures          []string                `json:"antiFeatures"`
	Name                  string                  `json:"name"`
	Summary               string                  `json:"summary"`
	Description           string                  `json:"description"`
	AllowedAPKSigningKeys []string                `json:"allowedAPKSigningKeys"`
	AuthorName            string                  `json:"authorName"`
	AuthorEmail           string                  `json:"authorEmail,omitempty"`
	AuthorWebSite         string                  `json:"authorWebSite,omitempty"`
	Categories            []string                `json:"categories"`
	Localized             map[string]LocalizedApp `json:"localized,omitempty"`
	PhoneScreenshots      []RepoFile              `json:"_rustorePhoneScreenshots,omitempty"`
	SuggestedVersionName  string                  `json:"suggestedVersionName"`
	SuggestedVersionCode  string                  `json:"suggestedVersionCode"`
	LastUpdated           int64                   `json:"lastUpdated"`
}

type LocalizedApp struct {
	Name             string   `json:"name,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Description      string   `json:"description,omitempty"`
	WhatsNew         string   `json:"whatsNew,omitempty"`
	PhoneScreenshots []string `json:"phoneScreenshots,omitempty"`
}

type RepoFile struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	SourceURL string `json:"sourceUrl,omitempty"`
}

type Package struct {
	PackageName         string   `json:"packageName"`
	Added               int64    `json:"added"`
	Size                int64    `json:"size"`
	APKName             string   `json:"apkName"`
	HashType            string   `json:"hashType"`
	Sig                 string   `json:"sig"`
	Signer              string   `json:"signer"`
	Signers             []string `json:"signers,omitempty"`
	HasMultipleSigners  bool     `json:"hasMultipleSigners,omitempty"`
	MinSdkVersion       int      `json:"minSdkVersion"`
	TargetSdkVersion    int      `json:"targetSdkVersion"`
	MaxSdkVersion       int      `json:"maxSdkVersion,omitempty"`
	VersionCode         int      `json:"versionCode"`
	VersionName         string   `json:"versionName"`
	Hash                string   `json:"hash"`
	NativeCode          []string `json:"nativecode,omitempty"`
	Features            []string `json:"features,omitempty"`
	UsesPermission      [][]any  `json:"uses-permission,omitempty"`
	UsesPermissionSDK23 [][]any  `json:"uses-permission-sdk-23,omitempty"`
	WhatsNew            string   `json:"whatsNew,omitempty"`
	APKMetadataVersion  int      `json:"apkMetadataVersion,omitempty"`
}

func JavaTime() int64 {
	return time.Now().UnixMilli()
}

func TimestrToTimestamp(timestr string) (int64, error) {
	if timestr == "" {
		return JavaTime(), nil
	}
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

func IndexV2Path(repoPath string) string {
	return filepath.Join(repoPath, "index-v2.json")
}

func EntryPath(repoPath string) string {
	return filepath.Join(repoPath, "entry.json")
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
	if err := loadPendingRemovals(repoPath, &idx); err != nil {
		return nil, err
	}

	return &idx, nil
}

func SaveIndexV1(repoPath string, idx *IndexV1) error {
	idx.Repo.Timestamp = JavaTime()
	idx.Repo.Version = MetadataVersion
	if idx.Repo.MaxAge == 0 {
		idx.Repo.MaxAge = DefaultMaxAge
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	if err := writeFileAtomic(IndexV1Path(repoPath), data, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}
	if err := savePendingRemovals(repoPath, idx.pendingRemovals); err != nil {
		return fmt.Errorf("write pending removals: %w", err)
	}

	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	tmpPath, err := stageFile(path, data, perm)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }()
	return os.Rename(tmpPath, path)
}

func stageFile(path string, data []byte, perm os.FileMode) (tmpPath string, err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return "", err
	}
	tmpPath = tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err = tmp.Chmod(perm); err != nil {
		return "", err
	}
	if _, err = tmp.Write(data); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	return tmpPath, nil
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
