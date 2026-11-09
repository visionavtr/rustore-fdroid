package cmd

import (
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [package_id...]",
	Short: "Update apps (all if no args given) and sign the repository",
	Long: "Update apps like the update command, then sign the repository like the sign command.\n" +
		"The repository is not signed when every app update fails.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := updatePackages(args); err != nil {
			return err
		}
		return signRepository(signCert, signKey)
	},
}

func init() {
	addSigningFlags(syncCmd)
	rootCmd.AddCommand(syncCmd)
}
