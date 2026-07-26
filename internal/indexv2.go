package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type IndexV2 struct {
	Repo     RepoV2               `json:"repo"`
	Packages map[string]PackageV2 `json:"packages"`
}

type RepoV2 struct {
	Name         map[string]string       `json:"name"`
	Icon         map[string]FileV2       `json:"icon,omitempty"`
	Address      string                  `json:"address"`
	WebBaseURL   string                  `json:"webBaseUrl,omitempty"`
	Description  map[string]string       `json:"description"`
	Timestamp    int64                   `json:"timestamp"`
	AntiFeatures map[string]DefinitionV2 `json:"antiFeatures,omitempty"`
	Categories   map[string]DefinitionV2 `json:"categories,omitempty"`
}

type DefinitionV2 struct {
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description,omitempty"`
}

type PackageV2 struct {
	Metadata MetadataV2                  `json:"metadata"`
	Versions map[string]PackageVersionV2 `json:"versions"`
}

type MetadataV2 struct {
	Name            map[string]string `json:"name,omitempty"`
	Summary         map[string]string `json:"summary,omitempty"`
	Description     map[string]string `json:"description,omitempty"`
	Added           int64             `json:"added"`
	LastUpdated     int64             `json:"lastUpdated"`
	License         string            `json:"license,omitempty"`
	PreferredSigner string            `json:"preferredSigner,omitempty"`
	Categories      []string          `json:"categories,omitempty"`
	AuthorName      string            `json:"authorName,omitempty"`
	AuthorEmail     string            `json:"authorEmail,omitempty"`
	AuthorWebSite   string            `json:"authorWebSite,omitempty"`
	Icon            map[string]FileV2 `json:"icon,omitempty"`
	Screenshots     *ScreenshotsV2    `json:"screenshots,omitempty"`
}

type ScreenshotsV2 struct {
	Phone map[string][]FileV2 `json:"phone,omitempty"`
}

type PackageVersionV2 struct {
	Added        int64                        `json:"added"`
	File         FileV2                       `json:"file"`
	Manifest     ManifestV2                   `json:"manifest"`
	AntiFeatures map[string]map[string]string `json:"antiFeatures,omitempty"`
	WhatsNew     map[string]string            `json:"whatsNew,omitempty"`
}

type FileV2 struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type ManifestV2 struct {
	NativeCode          []string       `json:"nativecode,omitempty"`
	VersionName         string         `json:"versionName"`
	MaxSdkVersion       int            `json:"maxSdkVersion,omitempty"`
	VersionCode         int            `json:"versionCode"`
	Features            []FeatureV2    `json:"features,omitempty"`
	UsesSDK             *UsesSDKV2     `json:"usesSdk,omitempty"`
	Signer              *SignerV2      `json:"signer,omitempty"`
	UsesPermission      []PermissionV2 `json:"usesPermission,omitempty"`
	UsesPermissionSDK23 []PermissionV2 `json:"usesPermissionSdk23,omitempty"`
}

type UsesSDKV2 struct {
	MinSdkVersion    int `json:"minSdkVersion"`
	TargetSdkVersion int `json:"targetSdkVersion"`
}

type SignerV2 struct {
	SHA256             []string `json:"sha256"`
	HasMultipleSigners bool     `json:"hasMultipleSigners,omitempty"`
}

type PermissionV2 struct {
	Name          string `json:"name"`
	MaxSdkVersion *int   `json:"maxSdkVersion,omitempty"`
}

type FeatureV2 struct {
	Name string `json:"name"`
}

type Entry struct {
	Timestamp int64                  `json:"timestamp"`
	Version   int                    `json:"version"`
	MaxAge    int                    `json:"maxAge,omitempty"`
	Index     EntryFileV2            `json:"index"`
	Diffs     map[string]EntryFileV2 `json:"diffs"`
}

type EntryFileV2 struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	NumPackages int    `json:"numPackages"`
}

func BuildIndexV2(repoPath string, idx *IndexV1) ([]byte, error) {
	webBaseURL := idx.Repo.WebBaseURL
	if webBaseURL == "" && idx.Repo.Address != "" {
		if _, err := os.Stat(filepath.Join(repoPath, "index.html")); err == nil {
			webBaseURL = strings.TrimRight(idx.Repo.Address, "/") + "/packages"
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check frontend: %w", err)
		}
	}

	v2 := IndexV2{
		Repo: RepoV2{
			Name:         localized(RepoLocale, idx.Repo.Name),
			Address:      idx.Repo.Address,
			WebBaseURL:   webBaseURL,
			Description:  localized(RepoLocale, idx.Repo.Description),
			Timestamp:    idx.Repo.Timestamp,
			AntiFeatures: make(map[string]DefinitionV2),
			Categories:   make(map[string]DefinitionV2),
		},
		Packages: make(map[string]PackageV2, len(idx.Apps)),
	}

	if icon, ok, err := repoIconFile(repoPath, idx.Repo.Icon); err != nil {
		return nil, err
	} else if ok {
		v2.Repo.Icon = map[string]FileV2{RepoLocale: icon}
	}

	for _, app := range idx.Apps {
		versions := idx.Packages[app.PackageName]
		if len(versions) == 0 {
			continue
		}

		metadata := MetadataV2{
			Name:          localized(AppLocale, app.Name),
			Summary:       localized(AppLocale, app.Summary),
			Description:   localized(AppLocale, app.Description),
			Added:         app.Added,
			LastUpdated:   app.LastUpdated,
			Categories:    app.Categories,
			AuthorName:    app.AuthorName,
			AuthorEmail:   app.AuthorEmail,
			AuthorWebSite: app.AuthorWebSite,
		}
		if app.License != "" && app.License != "Unknown" {
			metadata.License = app.License
		}

		if app.Icon != "" {
			icon, err := fileV2FromPath(repoPath, filepath.Join("icons", app.Icon))
			if err == nil {
				metadata.Icon = map[string]FileV2{AppLocale: icon}
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("app icon %s: %w", app.PackageName, err)
			}
		}

		if len(app.PhoneScreenshots) > 0 {
			files := make([]FileV2, 0, len(app.PhoneScreenshots))
			for _, screenshot := range app.PhoneScreenshots {
				file, err := storedFileV2(repoPath, screenshot)
				if err != nil {
					return nil, fmt.Errorf("screenshot %s: %w", app.PackageName, err)
				}
				files = append(files, file)
			}
			metadata.Screenshots = &ScreenshotsV2{Phone: map[string][]FileV2{AppLocale: files}}
		}

		packageV2 := PackageV2{
			Metadata: metadata,
			Versions: make(map[string]PackageVersionV2, len(versions)),
		}
		for _, pkg := range versions {
			decodedHash, err := hex.DecodeString(pkg.Hash)
			if err != nil || len(decodedHash) != sha256.Size {
				return nil, fmt.Errorf("package %s version %d has invalid SHA-256", app.PackageName, pkg.VersionCode)
			}
			apkInfo, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(pkg.APKName)))
			if err != nil {
				return nil, fmt.Errorf("package %s version %d: %w", app.PackageName, pkg.VersionCode, err)
			}
			if apkInfo.Size() != pkg.Size {
				return nil, fmt.Errorf("package %s version %d size is %d, index says %d", app.PackageName, pkg.VersionCode, apkInfo.Size(), pkg.Size)
			}

			signers := packageSigners(pkg)
			if packageV2.Metadata.PreferredSigner == "" && len(signers) > 0 {
				packageV2.Metadata.PreferredSigner = signers[0]
			}

			manifest := ManifestV2{
				NativeCode:          sortedCopy(pkg.NativeCode),
				VersionName:         pkg.VersionName,
				MaxSdkVersion:       pkg.MaxSdkVersion,
				VersionCode:         pkg.VersionCode,
				Features:            featuresV2(pkg.Features),
				UsesPermission:      permissionsV2(pkg.UsesPermission),
				UsesPermissionSDK23: permissionsV2(pkg.UsesPermissionSDK23),
			}
			if pkg.MinSdkVersion > 0 {
				target := pkg.TargetSdkVersion
				if target == 0 {
					target = pkg.MinSdkVersion
				}
				manifest.UsesSDK = &UsesSDKV2{MinSdkVersion: pkg.MinSdkVersion, TargetSdkVersion: target}
			}
			if len(signers) > 0 {
				manifest.Signer = &SignerV2{
					SHA256:             signers,
					HasMultipleSigners: pkg.HasMultipleSigners || len(signers) > 1,
				}
			}

			antiFeatures := make(map[string]map[string]string, len(app.AntiFeatures))
			for _, name := range app.AntiFeatures {
				antiFeatures[name] = map[string]string{}
				if _, ok := v2.Repo.AntiFeatures[name]; !ok {
					v2.Repo.AntiFeatures[name] = antiFeatureDefinition(name)
				}
			}

			version := PackageVersionV2{
				Added: pkg.Added,
				File: FileV2{
					Name:   "/" + strings.TrimPrefix(filepath.ToSlash(pkg.APKName), "/"),
					SHA256: pkg.Hash,
					Size:   pkg.Size,
				},
				Manifest:     manifest,
				AntiFeatures: antiFeatures,
				WhatsNew:     localized(AppLocale, pkg.WhatsNew),
			}
			packageV2.Versions[pkg.Hash] = version
		}

		for _, category := range app.Categories {
			if _, ok := v2.Repo.Categories[category]; !ok {
				v2.Repo.Categories[category] = DefinitionV2{Name: localized(AppLocale, category)}
			}
		}
		v2.Packages[app.PackageName] = packageV2
	}

	return json.Marshal(v2)
}

func BuildEntry(idx *IndexV1, indexData []byte, numPackages int) ([]byte, error) {
	hash := sha256.Sum256(indexData)
	maxAge := idx.Repo.MaxAge
	if maxAge == 0 {
		maxAge = DefaultMaxAge
	}
	entry := Entry{
		Timestamp: idx.Repo.Timestamp,
		Version:   MetadataVersion,
		MaxAge:    maxAge,
		Index: EntryFileV2{
			Name:        "/index-v2.json",
			SHA256:      hex.EncodeToString(hash[:]),
			Size:        int64(len(indexData)),
			NumPackages: numPackages,
		},
		Diffs: map[string]EntryFileV2{},
	}
	return json.Marshal(entry)
}

func IndexV2PackageCount(idx *IndexV1) int {
	count := 0
	for _, app := range idx.Apps {
		if len(idx.Packages[app.PackageName]) > 0 {
			count++
		}
	}
	return count
}

func localized(locale, value string) map[string]string {
	if value == "" {
		return nil
	}
	return map[string]string{locale: value}
}

func repoIconFile(repoPath, iconName string) (FileV2, bool, error) {
	if iconName == "" {
		return FileV2{}, false, nil
	}
	for _, name := range []string{filepath.Join("icons", iconName), iconName} {
		file, err := fileV2FromPath(repoPath, name)
		if err == nil {
			return file, true, nil
		}
		if !os.IsNotExist(err) {
			return FileV2{}, false, fmt.Errorf("repo icon: %w", err)
		}
	}
	return FileV2{}, false, nil
}

func fileV2FromPath(repoPath, name string) (FileV2, error) {
	path := filepath.Join(repoPath, filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		return FileV2{}, err
	}
	hash := sha256.Sum256(data)
	return FileV2{
		Name:   "/" + strings.TrimPrefix(filepath.ToSlash(name), "/"),
		SHA256: hex.EncodeToString(hash[:]),
		Size:   int64(len(data)),
	}, nil
}

func storedFileV2(repoPath string, stored RepoFile) (FileV2, error) {
	info, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(stored.Name)))
	if err != nil {
		return FileV2{}, err
	}
	if stored.SHA256 == "" || stored.Size != info.Size() {
		return fileV2FromPath(repoPath, stored.Name)
	}
	return FileV2{
		Name:   "/" + strings.TrimPrefix(filepath.ToSlash(stored.Name), "/"),
		SHA256: stored.SHA256,
		Size:   stored.Size,
	}, nil
}

func packageSigners(pkg Package) []string {
	if len(pkg.Signers) > 0 {
		return sortedCopy(pkg.Signers)
	}
	if pkg.Sig != "" {
		return []string{pkg.Sig}
	}
	if pkg.Signer != "" {
		return []string{pkg.Signer}
	}
	return nil
}

func featuresV2(features []string) []FeatureV2 {
	features = sortedCopy(features)
	result := make([]FeatureV2, 0, len(features))
	for _, name := range features {
		if name != "" {
			result = append(result, FeatureV2{Name: name})
		}
	}
	return result
}

func permissionsV2(permissions [][]any) []PermissionV2 {
	result := make([]PermissionV2, 0, len(permissions))
	for _, permission := range permissions {
		if len(permission) == 0 {
			continue
		}
		name, ok := permission[0].(string)
		if !ok || name == "" {
			continue
		}
		item := PermissionV2{Name: name}
		if len(permission) > 1 && permission[1] != nil {
			if maxSDK, ok := numberAsInt(permission[1]); ok && maxSDK > 0 {
				item.MaxSdkVersion = &maxSDK
			}
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func numberAsInt(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), value == float64(int(value))
	default:
		return 0, false
	}
}

func sortedCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func antiFeatureDefinition(name string) DefinitionV2 {
	if name == "NoSourceSince" {
		return DefinitionV2{
			Name:        localized(RepoLocale, "Source code no longer available"),
			Description: localized(RepoLocale, "The source code for this app is no longer publicly available."),
		}
	}
	return DefinitionV2{Name: localized(RepoLocale, name)}
}
