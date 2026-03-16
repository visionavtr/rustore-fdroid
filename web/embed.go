package web

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed index.html
var IndexHTML []byte

func Install(repoPath string) error {
	dst := filepath.Join(repoPath, "index.html")
	if err := os.WriteFile(dst, IndexHTML, 0o644); err != nil {
		return fmt.Errorf("install frontend: %w", err)
	}
	fmt.Println("Frontend installed.")
	return nil
}

func Remove(repoPath string) error {
	dst := filepath.Join(repoPath, "index.html")
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove frontend: %w", err)
	}
	fmt.Println("Frontend removed.")
	return nil
}
