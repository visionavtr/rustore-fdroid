package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
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

		for _, packageID := range args {
			fmt.Printf("--- %s ---\n", packageID)
			pf := prefetched[packageID]
			if pf.err != nil {
				fmt.Printf("Error adding %s: %v\n", packageID, pf.err)
				continue
			}
			if err := addPackageWithMeta(idx, pf.info, pf.dlInfo); err != nil {
				fmt.Printf("Error adding %s: %v\n", packageID, err)
				continue
			}
		}

		return internal.SaveIndexV1(repoPath, idx)
	},
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
	var err error
	var iconFile string
	if info.IconURL != "" {
		iconExt := iconExtFromURL(info.IconURL)
		iconOutput := filepath.Join(repoPath, "icons", info.PackageName+iconExt)
		iconFile, _, err = internal.DownloadAndGetSHA256(info.IconURL, iconOutput, 0)
		if err != nil {
			fmt.Printf("Warning: failed to download icon: %v\n", err)
		}
	}

	appIdx := internal.FindAppIndex(idx, info.PackageName)
	if appIdx == -1 {
		firstPublished, err := internal.TimestrToTimestamp(info.FirstPublishedAt)
		if err != nil {
			return fmt.Errorf("parse firstPublishedAt: %w", err)
		}
		idx.Apps = append(idx.Apps, internal.App{
			PackageName:  info.PackageName,
			Added:        firstPublished,
			Icon:         filepath.Base(iconFile),
			License:      "Unknown",
			AntiFeatures: []string{"NoSourceSince"},
		})
		appIdx = len(idx.Apps) - 1
	}

	lastUpdated, err := internal.TimestrToTimestamp(info.AppVerUpdatedAt)
	if err != nil {
		return fmt.Errorf("parse appVerUpdatedAt: %w", err)
	}

	idx.Apps[appIdx].Name = info.AppName
	idx.Apps[appIdx].Summary = info.ShortDescription
	idx.Apps[appIdx].Description = info.FullDescription
	idx.Apps[appIdx].AllowedAPKSigningKeys = info.Signatures
	idx.Apps[appIdx].AuthorName = info.CompanyName
	idx.Apps[appIdx].Categories = info.Categories
	idx.Apps[appIdx].SuggestedVersionName = info.VersionName
	idx.Apps[appIdx].SuggestedVersionCode = fmt.Sprintf("%d", info.VersionCode)
	idx.Apps[appIdx].LastUpdated = lastUpdated

	if !internal.PackageContainsVersion(idx, info.PackageName, info.VersionCode) {
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
			VersionCode:      info.VersionCode,
			VersionName:      info.VersionName,
		}

		// Check if APK already exists and matches xxhash
		if data, err := os.ReadFile(apkFile); err == nil {
			h := xxhash.Sum64(data)
			if fmt.Sprintf("%016x", h) == dlURL.Hash {
				hash := sha256.Sum256(data)
				indexPkg.Hash = hex.EncodeToString(hash[:])
			}
		}

		if indexPkg.Hash == "" {
			_, apkHash, err := internal.DownloadAndGetSHA256(dlURL.URL, apkFile, dlURL.Size)
			if err != nil {
				return fmt.Errorf("download APK: %w", err)
			}
			indexPkg.Hash = apkHash
		}

		sig, err := internal.ExtractAPKSig(apkFile)
		if err != nil {
			fmt.Printf("Warning: failed to extract APK signature: %v\n", err)
		}
		indexPkg.Sig = sig

		perms, err := internal.ExtractPermissions(apkFile)
		if err != nil {
			fmt.Printf("Warning: failed to extract permissions: %v\n", err)
		}
		indexPkg.UsesPermission = perms

		// Remove old versions (APK files + index entries)
		for _, old := range idx.Packages[info.PackageName] {
			oldPath := filepath.Join(repoPath, old.APKName)
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				fmt.Printf("Warning: failed to remove old APK %s: %v\n", old.APKName, err)
			} else if err == nil {
				fmt.Printf("Removed old version: %s\n", old.APKName)
			}
		}

		idx.Packages[info.PackageName] = []internal.Package{indexPkg}
	}

	// Backfill missing permissions from existing APKs on disk
	for i, pkg := range idx.Packages[info.PackageName] {
		if pkg.UsesPermission != nil {
			continue
		}
		apkFile := filepath.Join(repoPath, pkg.APKName)
		perms, err := internal.ExtractPermissions(apkFile)
		if err != nil {
			fmt.Printf("Warning: failed to extract permissions from %s: %v\n", pkg.APKName, err)
			continue
		}
		idx.Packages[info.PackageName][i].UsesPermission = perms
	}

	return nil
}

// iconExtFromURL extracts a file extension from the icon URL path.
// Falls back to ".png" if the URL has no recognizable image extension.
func iconExtFromURL(rawURL string) string {
	ext := filepath.Ext(rawURL)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg":
		return ext
	default:
		return ".png"
	}
}

func init() {
	rootCmd.AddCommand(addCmd)
}
