package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
)

func DownloadAndGetSHA256(url, output string, size int64) (string, string, error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", "", fmt.Errorf("create directory: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(output)
	if err != nil {
		return "", "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	hash := sha256.New()

	if size <= 0 {
		size = resp.ContentLength
	}
	if size <= 0 {
		size = -1
	}

	bar := progressbar.DefaultBytes(size, "Downloading "+filepath.Base(output))
	writer := io.MultiWriter(f, hash, bar)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return "", "", fmt.Errorf("write file: %w", err)
	}

	return output, hex.EncodeToString(hash.Sum(nil)), nil
}
