package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List packages in repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}

		for _, app := range idx.Apps {
			fmt.Printf("%s | %s\n", app.PackageName, app.Name)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
