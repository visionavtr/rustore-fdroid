package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var Version = "dev"

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
}

var (
	repoPath          string
	downloadChunkSize int
)

var rootCmd = &cobra.Command{
	Use:     "rustore-fdroid",
	Short:   "RuStore to F-Droid bridge — generate F-Droid repos with apps from RuStore",
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			return fmt.Errorf("create repo directory: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&repoPath, "repo", "r", "", "repository path")
	rootCmd.PersistentFlags().IntVar(&downloadChunkSize, "download-chunk-size", 128*1024, "chunk size for file downloads")
	_ = rootCmd.MarkPersistentFlagRequired("repo")
}

func Execute() error {
	return rootCmd.Execute()
}
