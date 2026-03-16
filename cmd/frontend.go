package cmd

import (
	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/web"
)

var frontendCmd = &cobra.Command{
	Use:   "frontend",
	Short: "Manage web frontend in repository",
}

var frontendAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Install web frontend into repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return web.Install(repoPath)
	},
}

var frontendRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove web frontend from repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return web.Remove(repoPath)
	},
}

func init() {
	frontendCmd.AddCommand(frontendAddCmd)
	frontendCmd.AddCommand(frontendRemoveCmd)
	rootCmd.AddCommand(frontendCmd)
}
