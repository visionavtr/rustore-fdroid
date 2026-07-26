package web

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/visionavtr/rustore-fdroid/internal"
	qrcode "github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

//go:embed index.html
var IndexHTML []byte

const generatedPackagesFile = ".rustore-fdroid-web-packages.json"

type RepoInfo struct {
	Address     string `json:"address"`
	WebBaseURL  string `json:"webBaseUrl"`
	Fingerprint string `json:"fingerprint"`
	AddURL      string `json:"addUrl"`
	FDroidLink  string `json:"fdroidLink"`
}

type bufferWriteCloser struct {
	bytes.Buffer
}

func (buffer *bufferWriteCloser) Close() error {
	return nil
}

func Install(repoPath string, idx *internal.IndexV1) error {
	dst := filepath.Join(repoPath, "index.html")
	if err := writeFileAtomic(dst, IndexHTML, 0o644); err != nil {
		return fmt.Errorf("install frontend: %w", err)
	}
	if err := Refresh(repoPath, idx); err != nil {
		return err
	}
	fmt.Println("Frontend installed.")
	return nil
}

func Refresh(repoPath string, idx *internal.IndexV1) error {
	if _, err := os.Stat(filepath.Join(repoPath, "index.html")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("check frontend: %w", err)
	}

	var previous []string
	if data, err := os.ReadFile(filepath.Join(repoPath, generatedPackagesFile)); err == nil {
		_ = json.Unmarshal(data, &previous)
	}

	current := make([]string, 0, len(idx.Apps))
	currentSet := make(map[string]struct{}, len(idx.Apps))
	for _, app := range idx.Apps {
		if err := internal.ValidatePackageName(app.PackageName); err != nil {
			return err
		}
		current = append(current, app.PackageName)
		currentSet[app.PackageName] = struct{}{}
		pageDir := filepath.Join(repoPath, "packages", app.PackageName)
		if err := os.MkdirAll(pageDir, 0o755); err != nil {
			return fmt.Errorf("create app page directory: %w", err)
		}
		page := appRedirectPage(app.PackageName, app.Name)
		if err := writeFileAtomic(filepath.Join(pageDir, "index.html"), page, 0o644); err != nil {
			return fmt.Errorf("write app page: %w", err)
		}
	}

	for _, packageName := range previous {
		if _, ok := currentSet[packageName]; ok {
			continue
		}
		if internal.ValidatePackageName(packageName) == nil {
			_ = os.Remove(filepath.Join(repoPath, "packages", packageName, "index.html"))
			_ = os.Remove(filepath.Join(repoPath, "packages", packageName))
		}
	}
	manifest, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("marshal app page manifest: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(repoPath, generatedPackagesFile), manifest, 0o644); err != nil {
		return fmt.Errorf("write app page manifest: %w", err)
	}
	return nil
}

func WriteSigningInfo(repoPath string, idx *internal.IndexV1, fingerprint string) error {
	if _, err := os.Stat(filepath.Join(repoPath, "index.html")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("check frontend: %w", err)
	}

	addURL, err := fingerprintURL(idx.Repo.Address, fingerprint)
	if err != nil {
		return err
	}
	webBaseURL := idx.Repo.WebBaseURL
	if webBaseURL == "" {
		webBaseURL = strings.TrimRight(idx.Repo.Address, "/") + "/packages"
	}
	info := RepoInfo{
		Address:     idx.Repo.Address,
		WebBaseURL:  webBaseURL,
		Fingerprint: fingerprint,
		AddURL:      addURL,
		FDroidLink:  "https://fdroid.link/#" + addURL,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal repository info: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(repoPath, "repo-info.json"), data, 0o644); err != nil {
		return fmt.Errorf("write repository info: %w", err)
	}

	code, err := qrcode.New(addURL)
	if err != nil {
		return fmt.Errorf("create repository QR code: %w", err)
	}
	buffer := &bufferWriteCloser{}
	writer := standard.NewWithWriter(
		buffer,
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
		standard.WithQRWidth(8),
		standard.WithBorderWidth(24),
	)
	if err := code.Save(writer); err != nil {
		return fmt.Errorf("render repository QR code: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close repository QR code: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(repoPath, "index.png"), buffer.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write repository QR code: %w", err)
	}
	return Refresh(repoPath, idx)
}

func Remove(repoPath string) error {
	var packages []string
	if data, err := os.ReadFile(filepath.Join(repoPath, generatedPackagesFile)); err == nil {
		_ = json.Unmarshal(data, &packages)
	}
	for _, packageName := range packages {
		if internal.ValidatePackageName(packageName) == nil {
			_ = os.Remove(filepath.Join(repoPath, "packages", packageName, "index.html"))
			_ = os.Remove(filepath.Join(repoPath, "packages", packageName))
		}
	}
	_ = os.Remove(filepath.Join(repoPath, "packages"))

	for _, name := range []string{"index.html", "index.png", "repo-info.json", generatedPackagesFile} {
		if err := os.Remove(filepath.Join(repoPath, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove frontend file %s: %w", name, err)
		}
	}
	fmt.Println("Frontend removed.")
	return nil
}

func appRedirectPage(packageName, appName string) []byte {
	query := url.Values{"package": []string{packageName}}.Encode()
	target := "../../?" + query
	return []byte("<!doctype html><html><head><meta charset=\"utf-8\">" +
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
		"<meta http-equiv=\"refresh\" content=\"0;url=" + html.EscapeString(target) + "\">" +
		"<title>" + html.EscapeString(appName) + "</title></head><body>" +
		"<script>location.replace(" + strconv.Quote(target) + ")</script>" +
		"<a href=\"" + html.EscapeString(target) + "\">Open " + html.EscapeString(appName) + "</a>" +
		"</body></html>")
}

func fingerprintURL(address, fingerprint string) (string, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("parse repository address: %w", err)
	}
	query := parsed.Query()
	query.Set("fingerprint", fingerprint)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err = tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
