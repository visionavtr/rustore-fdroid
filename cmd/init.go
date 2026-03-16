package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
	"github.com/visionavtr/rustore-fdroid/web"
)

var (
	initName        string
	initDescription string
	initAddress     string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize empty F-Droid repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		idx := &internal.IndexV1{
			Repo: internal.Repo{
				Timestamp:   internal.JavaTime(),
				Version:     0,
				Name:        initName,
				Icon:        "icon.jpg",
				Address:     initAddress,
				Description: initDescription,
			},
			Requests: internal.Requests{
				Install:   []any{},
				Uninstall: []any{},
			},
			Apps:     []internal.App{},
			Packages: make(map[string][]internal.Package),
		}

		if err := internal.SaveIndexV1(repoPath, idx); err != nil {
			return err
		}

		withFrontend, _ := cmd.Flags().GetBool("frontend")
		if withFrontend {
			if err := web.Install(repoPath); err != nil {
				return err
			}
		}

		fmt.Printf("New empty repository initialized at %s!\n", repoPath)
		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "repository name")
	initCmd.Flags().StringVarP(&initDescription, "description", "d", "", "repository description")
	initCmd.Flags().StringVarP(&initAddress, "address", "a", "", "repository address (URL)")
	_ = initCmd.MarkFlagRequired("name")
	_ = initCmd.MarkFlagRequired("description")
	_ = initCmd.MarkFlagRequired("address")
	initCmd.Flags().Bool("frontend", false, "include web frontend (index.html)")
	rootCmd.AddCommand(initCmd)
}
