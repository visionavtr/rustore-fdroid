package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
)

var addCmd = &cobra.Command{
	Use:   "add <package_id> [package_id...]",
	Short: "Add apps from RuStore",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}

		for _, packageID := range args {
			fmt.Printf("--- %s ---\n", packageID)
			if err := addPackage(idx, packageID); err != nil {
				fmt.Printf("Error adding %s: %v\n", packageID, err)
				continue
			}
		}

		return internal.SaveIndexV1(repoPath, idx)
	},
}

func addPackage(idx *internal.IndexV1, packageID string) error {
	info, err := internal.FetchAppInfo(packageID)
	if err != nil {
		return err
	}

	dlInfo, err := internal.FetchDownloadLink(info.AppID)
	if err != nil {
		return err
	}

	var iconFile string
	if info.IconURL != "" {
		iconOutput := filepath.Join(repoPath, "icons", info.PackageName+".icon.jpg")
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
			return fmt.Errorf("no download URLs available for %s", packageID)
		}
		dlURL := dlInfo.DownloadURLs[0]
		apkFile := filepath.Join(repoPath, fmt.Sprintf("%s_%d.apk", info.PackageName, info.VersionCode))

		indexPkg := internal.Package{
			PackageName:      info.PackageName,
			Added:            lastUpdated,
			Size:             dlURL.Size,
			APKName:          filepath.Base(apkFile),
			HashType:         "sha256",
			Sig:              "deadbeef", // FIXME: implement sig
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

		idx.Packages[info.PackageName] = append(idx.Packages[info.PackageName], indexPkg)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)
}
