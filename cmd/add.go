package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
	"github.com/visionavtr/rustore-fdroid/web"
)

type prefetchResult struct {
	info   *internal.AppInfo
	dlInfo *internal.DownloadBody
	err    error
}

var addCmd = &cobra.Command{
	Use:   "add <package_id> [package_id...]",
	Short: "Add apps from RuStore",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}

		prefetched := prefetchMetadata(args)
		succeeded := 0
		var failures []error

		for _, packageID := range args {
			fmt.Printf("--- %s ---\n", packageID)
			pf := prefetched[packageID]
			if pf.err != nil {
				fmt.Printf("Error adding %s: %v\n", packageID, pf.err)
				failures = append(failures, fmt.Errorf("%s: %w", packageID, pf.err))
				continue
			}
			if err := addPackageWithMeta(idx, pf.info, pf.dlInfo); err != nil {
				fmt.Printf("Error adding %s: %v\n", packageID, err)
				failures = append(failures, fmt.Errorf("%s: %w", packageID, err))
				continue
			}
			succeeded++
		}

		return finishPackageBatch(idx, "add", len(args), succeeded, failures)
	},
}

func finishPackageBatch(idx *internal.IndexV1, operation string, total, succeeded int, failures []error) error {
	if succeeded == 0 {
		return fmt.Errorf("failed to %s all %d packages: %w", operation, total, errors.Join(failures...))
	}
	if err := internal.SaveIndexV1(repoPath, idx); err != nil {
		return err
	}
	return web.Refresh(repoPath, idx)
}

// maxConcurrentFetches limits parallel API requests to avoid overwhelming the server.
const maxConcurrentFetches = 4

func prefetchMetadata(packageIDs []string) map[string]prefetchResult {
	results := make(map[string]prefetchResult, len(packageIDs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentFetches)

	for _, pkg := range packageIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := internal.FetchAppInfo(id)
			if err != nil {
				mu.Lock()
				results[id] = prefetchResult{err: err}
				mu.Unlock()
				return
			}
			dlInfo, err := internal.FetchDownloadLink(info.AppID)
			mu.Lock()
			results[id] = prefetchResult{info: info, dlInfo: dlInfo, err: err}
			mu.Unlock()
		}(pkg)
	}
	wg.Wait()
	return results
}

// addPackage fetches metadata and adds a package (used by update command).
func addPackage(idx *internal.IndexV1, packageID string) error {
	pf := prefetchMetadata([]string{packageID})
	r := pf[packageID]
	if r.err != nil {
		return r.err
	}
	return addPackageWithMeta(idx, r.info, r.dlInfo)
}

func addPackageWithMeta(idx *internal.IndexV1, info *internal.AppInfo, dlInfo *internal.DownloadBody) error {
	if err := internal.ValidatePackageName(info.PackageName); err != nil {
		return err
	}

	appIdx := internal.FindAppIndex(idx, info.PackageName)
	firstPublished := int64(0)
	if appIdx == -1 {
		var err error
		firstPublished, err = internal.TimestrToTimestamp(info.FirstPublishedAt)
		if err != nil {
			return fmt.Errorf("parse firstPublishedAt: %w", err)
		}
	}

	lastUpdated, err := internal.TimestrToTimestamp(info.AppVerUpdatedAt)
	if err != nil {
		return fmt.Errorf("parse appVerUpdatedAt: %w", err)
	}

	if !internal.PackageContainsVersion(idx, info.PackageName, info.VersionCode) {
		if dlInfo == nil {
			return fmt.Errorf("missing download metadata for %s", info.PackageName)
		}
		if len(dlInfo.DownloadURLs) == 0 {
			return fmt.Errorf("no download URLs available for %s", info.PackageName)
		}
		dlURL := dlInfo.DownloadURLs[0]
		apkFile := filepath.Join(repoPath, fmt.Sprintf("%s_%d.apk", info.PackageName, info.VersionCode))

		indexPkg := internal.Package{
			PackageName:      info.PackageName,
			Added:            lastUpdated,
			Size:             dlURL.Size,
			APKName:          filepath.Base(apkFile),
			HashType:         "sha256",
			Signer:           dlInfo.Signature,
			MinSdkVersion:    info.MinSdkVersion,
			TargetSdkVersion: info.TargetSdkVersion,
			MaxSdkVersion:    info.MaxSdkVersion,
			VersionCode:      info.VersionCode,
			VersionName:      info.VersionName,
			WhatsNew:         info.WhatsNew,
		}

		// Check if APK already exists and matches xxhash
		downloaded := false
		if dlURL.Hash != "" {
			if sha, xxh, err := internal.FileHashes(apkFile); err == nil && xxh == dlURL.Hash {
				indexPkg.Hash = sha
			}
		}

		if indexPkg.Hash == "" {
			_, apkHash, err := internal.DownloadAndGetSHA256(dlURL.URL, apkFile, dlURL.Size)
			if err != nil {
				return fmt.Errorf("download APK: %w", err)
			}
			indexPkg.Hash = apkHash
			downloaded = true
		}
		fileInfo, err := os.Stat(apkFile)
		if err != nil {
			return fmt.Errorf("stat APK: %w", err)
		}
		indexPkg.Size = fileInfo.Size()

		metadata, err := internal.ExtractAPKMetadata(apkFile)
		if err != nil {
			if downloaded {
				_ = os.Remove(apkFile)
			}
			return fmt.Errorf("extract APK metadata: %w", err)
		}
		if metadata.PackageName != info.PackageName {
			if downloaded {
				_ = os.Remove(apkFile)
			}
			return fmt.Errorf("downloaded APK package is %q, want %q", metadata.PackageName, info.PackageName)
		}
		if metadata.VersionCode != info.VersionCode {
			if downloaded {
				_ = os.Remove(apkFile)
			}
			return fmt.Errorf("downloaded APK version code is %d, want %d", metadata.VersionCode, info.VersionCode)
		}
		if dlInfo.Signature != "" && !containsFold(metadata.Signers, dlInfo.Signature) {
			if downloaded {
				_ = os.Remove(apkFile)
			}
			return fmt.Errorf("downloaded APK signer does not match RuStore signature")
		}
		if len(info.Signatures) > 0 && !containsAnyFold(metadata.Signers, info.Signatures) {
			if downloaded {
				_ = os.Remove(apkFile)
			}
			return fmt.Errorf("downloaded APK signer is not allowed by RuStore metadata")
		}
		applyAPKMetadata(&indexPkg, metadata)

		// Keep old files until the new indexes have been signed successfully.
		for _, old := range idx.Packages[info.PackageName] {
			if err := internal.QueueFileRemoval(idx, old.APKName); err != nil {
				return err
			}
			fmt.Printf("Old version will be removed after signing: %s\n", old.APKName)
		}

		idx.Packages[info.PackageName] = []internal.Package{indexPkg}
	}

	if appIdx == -1 {
		idx.Apps = append(idx.Apps, internal.App{
			PackageName:  info.PackageName,
			Added:        firstPublished,
			License:      "Unknown",
			AntiFeatures: []string{"NoSourceSince"},
		})
		appIdx = len(idx.Apps) - 1
	}

	app := &idx.Apps[appIdx]
	app.Name = info.AppName
	app.Summary = info.ShortDescription
	app.Description = info.FullDescription
	app.AllowedAPKSigningKeys = info.Signatures
	app.AuthorName = info.CompanyName
	app.AuthorEmail = info.DeveloperContacts.Email
	app.AuthorWebSite = info.DeveloperContacts.Website
	app.Categories = info.Categories
	app.SuggestedVersionName = info.VersionName
	app.SuggestedVersionCode = fmt.Sprintf("%d", info.VersionCode)
	app.LastUpdated = lastUpdated

	if info.IconURL != "" {
		iconName := info.PackageName + iconExtFromURL(info.IconURL)
		iconOutput := filepath.Join(repoPath, "icons", iconName)
		if _, _, err := internal.DownloadAndGetSHA256(info.IconURL, iconOutput, 0); err != nil {
			fmt.Printf("Warning: failed to download icon: %v\n", err)
		} else {
			oldIcon := app.Icon
			app.Icon = iconName
			if oldIcon != "" && oldIcon != iconName {
				if err := internal.QueueFileRemoval(idx, filepath.Join("icons", oldIcon)); err != nil {
					return err
				}
			}
		}
	}
	if info.FileURLs != nil {
		app.PhoneScreenshots, err = syncPhoneScreenshots(idx, info, app.PhoneScreenshots)
		if err != nil {
			return err
		}
	}
	if app.Localized == nil {
		app.Localized = make(map[string]internal.LocalizedApp)
	}
	localized := internal.LocalizedApp{
		Name:        info.AppName,
		Summary:     info.ShortDescription,
		Description: info.FullDescription,
		WhatsNew:    info.WhatsNew,
	}
	for _, screenshot := range app.PhoneScreenshots {
		localized.PhoneScreenshots = append(localized.PhoneScreenshots, filepath.Base(screenshot.Name))
	}
	app.Localized[internal.AppLocale] = localized

	// Backfill v2 compatibility metadata from existing APKs on disk once.
	for i, pkg := range idx.Packages[info.PackageName] {
		if pkg.APKMetadataVersion >= internal.CurrentAPKMetadataVersion {
			continue
		}
		apkFile := filepath.Join(repoPath, pkg.APKName)
		metadata, err := internal.ExtractAPKMetadata(apkFile)
		if err != nil {
			fmt.Printf("Warning: failed to extract metadata from %s: %v\n", pkg.APKName, err)
			continue
		}
		if metadata.PackageName != info.PackageName || metadata.VersionCode != pkg.VersionCode {
			fmt.Printf("Warning: APK identity mismatch for %s\n", pkg.APKName)
			continue
		}
		applyAPKMetadata(&idx.Packages[info.PackageName][i], metadata)
	}
	for i := range idx.Packages[info.PackageName] {
		if idx.Packages[info.PackageName][i].VersionCode == info.VersionCode {
			idx.Packages[info.PackageName][i].WhatsNew = info.WhatsNew
		}
	}

	return nil
}

// iconExtFromURL extracts a file extension from the icon URL path.
// Falls back to ".png" if the URL has no recognizable image extension.
func iconExtFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ".png"
	}
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg":
		return ext
	default:
		return ".png"
	}
}

func syncPhoneScreenshots(idx *internal.IndexV1, info *internal.AppInfo, existing []internal.RepoFile) ([]internal.RepoFile, error) {
	files := make([]internal.AppFile, 0, len(info.FileURLs))
	for _, file := range info.FileURLs {
		if strings.EqualFold(file.Type, "SCREENSHOT") && file.URL != "" {
			files = append(files, file)
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Ordinal < files[j].Ordinal })

	existingByName := make(map[string]internal.RepoFile, len(existing))
	for _, file := range existing {
		existingByName[file.Name] = file
	}
	result := make([]internal.RepoFile, 0, len(files))
	kept := make(map[string]struct{}, len(files))
	for i, source := range files {
		name := filepath.ToSlash(filepath.Join(
			info.PackageName,
			internal.AppLocale,
			"phoneScreenshots",
			fmt.Sprintf("%02d%s", i+1, iconExtFromURL(source.URL)),
		))
		old, hasOld := existingByName[name]
		output := filepath.Join(repoPath, filepath.FromSlash(name))
		if hasOld && old.SourceURL == source.URL {
			if fileInfo, err := os.Stat(output); err == nil && fileInfo.Size() == old.Size {
				result = append(result, old)
				kept[name] = struct{}{}
				continue
			}
		}

		_, hash, err := internal.DownloadAndGetSHA256(source.URL, output, 0)
		if err != nil {
			fmt.Printf("Warning: failed to download screenshot %s: %v\n", source.URL, err)
			if hasOld {
				if fileInfo, statErr := os.Stat(output); statErr == nil && fileInfo.Size() == old.Size {
					result = append(result, old)
					kept[name] = struct{}{}
				}
			}
			continue
		}
		fileInfo, err := os.Stat(output)
		if err != nil {
			fmt.Printf("Warning: failed to stat screenshot %s: %v\n", name, err)
			continue
		}
		result = append(result, internal.RepoFile{
			Name:      name,
			SHA256:    hash,
			Size:      fileInfo.Size(),
			SourceURL: source.URL,
		})
		kept[name] = struct{}{}
	}

	for _, old := range existing {
		if _, ok := kept[old.Name]; !ok {
			if err := internal.QueueFileRemoval(idx, old.Name); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func applyAPKMetadata(pkg *internal.Package, metadata *internal.APKMetadata) {
	if metadata.MinSDKVersion > 0 {
		pkg.MinSdkVersion = metadata.MinSDKVersion
	}
	if metadata.TargetSDKVersion > 0 {
		pkg.TargetSdkVersion = metadata.TargetSDKVersion
	}
	if metadata.MaxSDKVersion > 0 {
		pkg.MaxSdkVersion = metadata.MaxSDKVersion
	}
	pkg.NativeCode = metadata.NativeCode
	pkg.Features = metadata.Features
	pkg.UsesPermission = metadata.UsesPermission
	pkg.UsesPermissionSDK23 = metadata.UsesPermissionSDK23
	pkg.Signers = metadata.Signers
	pkg.HasMultipleSigners = metadata.HasMultipleSigners
	if len(metadata.Signers) > 0 {
		pkg.Sig = metadata.Signers[0]
	}
	pkg.APKMetadataVersion = internal.CurrentAPKMetadataVersion
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}

func containsAnyFold(left, right []string) bool {
	for _, value := range right {
		if containsFold(left, value) {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(addCmd)
}
