package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/visionavtr/rustore-fdroid/internal"
)

var (
	signKey  string
	signCert string
)

var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign repository (generates index-v1.jar)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := internal.SignJAR(repoPath, signCert, signKey); err != nil {
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
