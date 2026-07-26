package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
	"github.com/visionavtr/rustore-fdroid/web"
)

var keepFiles bool

var removeCmd = &cobra.Command{
	Use:   "remove <package_id> [package_id...]",
	Short: "Remove apps from repository",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}

		for _, packageID := range args {
			appIdx := internal.FindAppIndex(idx, packageID)
			if appIdx == -1 {
				fmt.Printf("Application %s is not in repository!\n", packageID)
				continue
			}

			app := idx.Apps[appIdx]

			if !keepFiles {
				if app.Icon != "" {
					if err := internal.QueueFileRemoval(idx, filepath.Join("icons", app.Icon)); err != nil {
						return err
					}
				}
				for _, pkg := range idx.Packages[app.PackageName] {
					if err := internal.QueueFileRemoval(idx, pkg.APKName); err != nil {
						return err
					}
				}
				for _, screenshot := range app.PhoneScreenshots {
					if err := internal.QueueFileRemoval(idx, screenshot.Name); err != nil {
						return err
					}
				}
			}

			delete(idx.Packages, app.PackageName)
			idx.Apps = append(idx.Apps[:appIdx], idx.Apps[appIdx+1:]...)
		}

		if err := internal.SaveIndexV1(repoPath, idx); err != nil {
			return err
		}
		return web.Refresh(repoPath, idx)
	},
}

func init() {
	removeCmd.Flags().BoolVarP(&keepFiles, "keep-files", "k", false, "keep icon and APK files")
	rootCmd.AddCommand(removeCmd)
}
