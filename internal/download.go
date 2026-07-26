package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
	"github.com/schollz/progressbar/v3"
)

func DownloadAndGetSHA256(url, output string, size int64) (path, sha256Hex string, err error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", "", fmt.Errorf("create directory: %w", err)
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp(filepath.Dir(output), "."+filepath.Base(output)+"-*")
	if err != nil {
		return "", "", fmt.Errorf("create temporary file: %w", err)
	}
	tmpPath := f.Name()
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	hash := sha256.New()

	if size <= 0 {
		size = resp.ContentLength
	}
	if size <= 0 {
		size = -1
	}

	bar := progressbar.DefaultBytes(size, "Downloading "+filepath.Base(output))
	written, err := io.Copy(io.MultiWriter(f, hash, bar), resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("write file: %w", err)
	}
	if size > 0 && written != size {
		return "", "", fmt.Errorf("download size mismatch: got %d bytes, want %d", written, size)
	}
	if err := f.Sync(); err != nil {
		return "", "", fmt.Errorf("sync file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", "", fmt.Errorf("close file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return "", "", fmt.Errorf("set file permissions: %w", err)
	}
	if err := os.Rename(tmpPath, output); err != nil {
		return "", "", fmt.Errorf("publish file: %w", err)
	}

	return output, hex.EncodeToString(hash.Sum(nil)), nil
}

func FileHashes(path string) (sha256Hex, xxhashHex string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	sha := sha256.New()
	xxh := xxhash.New()
	if _, err := io.Copy(io.MultiWriter(sha, xxh), f); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(sha.Sum(nil)), fmt.Sprintf("%016x", xxh.Sum64()), nil
}
