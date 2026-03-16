package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
)

var updateCmd = &cobra.Command{
	Use:   "update [package_id...]",
	Short: "Update apps in repository (all if no args given)",
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}

		if len(args) == 0 {
			for _, app := range idx.Apps {
				args = append(args, app.PackageName)
			}
		}

		if len(args) == 0 {
			fmt.Println("Repository is empty, nothing to update.")
			return nil
		}

		prefetched := prefetchMetadata(args)

		for _, packageID := range args {
			fmt.Printf("--- %s ---\n", packageID)
			pf := prefetched[packageID]
			if pf.err != nil {
				fmt.Printf("Error updating %s: %v\n", packageID, pf.err)
				continue
			}
			if err := addPackageWithMeta(idx, pf.info, pf.dlInfo); err != nil {
				fmt.Printf("Error updating %s: %v\n", packageID, err)
				continue
			}
		}

		return internal.SaveIndexV1(repoPath, idx)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
