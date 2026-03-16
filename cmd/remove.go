package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
)

var keepFiles bool

var removeCmd = &cobra.Command{
	Use:   "remove <package_id>",
	Short: "Remove app from repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		packageID := args[0]

		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}

		appIdx := internal.FindAppIndex(idx, packageID)
		if appIdx == -1 {
			fmt.Printf("Application %s is not in repository!\n", packageID)
			return nil
		}

		app := idx.Apps[appIdx]

		if !keepFiles {
			os.Remove(filepath.Join(repoPath, "icons", app.Icon))
			for _, pkg := range idx.Packages[app.PackageName] {
				os.Remove(filepath.Join(repoPath, pkg.APKName))
			}
		}

		delete(idx.Packages, app.PackageName)
		idx.Apps = append(idx.Apps[:appIdx], idx.Apps[appIdx+1:]...)

		return internal.SaveIndexV1(repoPath, idx)
	},
}

func init() {
	removeCmd.Flags().BoolVarP(&keepFiles, "keep-files", "k", false, "keep icon and APK files")
	rootCmd.AddCommand(removeCmd)
}
