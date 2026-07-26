package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
	"github.com/visionavtr/rustore-fdroid/web"
)

var (
	signKey  string
	signCert string
)

var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign repository (generates index-v1.jar and entry.jar)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fingerprint, err := internal.SignRepository(repoPath, signCert, signKey)
		if err != nil {
			return err
		}
		idx, err := internal.LoadIndexV1(repoPath)
		if err != nil {
			return err
		}
		if err := web.WriteSigningInfo(repoPath, idx, fingerprint); err != nil {
			return err
		}
		if err := internal.CommitPendingRemovals(repoPath); err != nil {
			return err
		}
		fmt.Println("Repository signed successfully.")
		return nil
	},
}

func init() {
	signCmd.Flags().StringVarP(&signKey, "key", "k", "", "path to private key (PEM)")
	signCmd.Flags().StringVarP(&signCert, "cert", "c", "", "path to certificate (PEM)")
	_ = signCmd.MarkFlagRequired("key")
	_ = signCmd.MarkFlagRequired("cert")
	rootCmd.AddCommand(signCmd)
}
