package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

const rustoreBaseURL = "https://backapi.rustore.ru/applicationData"

// Mutable for testing.
var (
	fetchAppInfoURL      = rustoreBaseURL + "/overallInfo/"
	fetchDownloadLinkURL = rustoreBaseURL + "/v2/download-link"
)

type rustoreResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type OverallInfoResponse struct {
	rustoreResponse
	Body AppInfo `json:"body"`
}

type AppInfo struct {
	AppID            int      `json:"appId"`
	PackageName      string   `json:"packageName"`
	AppName          string   `json:"appName"`
	ShortDescription string   `json:"shortDescription"`
	FullDescription  string   `json:"fullDescription"`
	IconURL          string   `json:"iconUrl"`
	VersionCode      int      `json:"versionCode"`
	VersionName      string   `json:"versionName"`
	MinSdkVersion    int      `json:"minSdkVersion"`
	TargetSdkVersion int      `json:"targetSdkVersion"`
	CompanyName      string   `json:"companyName"`
	Categories       []string `json:"categories"`
	Signatures       []string `json:"signatures"`
	FirstPublishedAt string   `json:"firstPublishedAt"`
	AppVerUpdatedAt  string   `json:"appVerUpdatedAt"`
}

type DownloadLinkResponse struct {
	Body DownloadBody `json:"body"`
}

type DownloadBody struct {
	DownloadURLs []DownloadURL `json:"downloadUrls"`
	Signature    string        `json:"signature"`
}

type DownloadURL struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
}

// httpClient is a shared HTTP client with a reasonable timeout.
// The timeout covers the entire request lifecycle including body download,
// so it must be generous enough for large APK files (hundreds of MB).
var httpClient = &http.Client{Timeout: 10 * time.Minute}

// validPackageName matches a valid Android package name (Java-style identifier).
var validPackageName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)

// ValidatePackageName checks that the name looks like a real Android package ID
// and cannot be used for path traversal.
func ValidatePackageName(name string) error {
	if name == "" {
		return fmt.Errorf("empty package name")
	}
	if !validPackageName.MatchString(name) {
		return fmt.Errorf("invalid package name %q", name)
	}
	return nil
}

func FetchAppInfo(packageID string) (*AppInfo, error) {
	return WithRetry("fetch app info", func() (*AppInfo, error) {
		return fetchAppInfo(packageID)
	})
}

func fetchAppInfo(packageID string) (*AppInfo, error) {
	resp, err := httpClient.Get(fetchAppInfoURL + packageID)
	if err != nil {
		return nil, fmt.Errorf("fetch app info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch app info: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result OverallInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse app info: %w", err)
	}

	if result.Code == "ERROR" {
		return nil, fmt.Errorf("app %q not found in RuStore", packageID)
	}

	if err := ValidatePackageName(result.Body.PackageName); err != nil {
		return nil, fmt.Errorf("rustore returned %w", err)
	}

	return &result.Body, nil
}

func FetchDownloadLink(appID int) (*DownloadBody, error) {
	return WithRetry("fetch download link", func() (*DownloadBody, error) {
		return fetchDownloadLink(appID)
	})
}

func fetchDownloadLink(appID int) (*DownloadBody, error) {
	payload, _ := json.Marshal(map[string]int{"appId": appID})

	resp, err := httpClient.Post(fetchDownloadLinkURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("fetch download link: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch download link: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result DownloadLinkResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse download link: %w", err)
	}

	return &result.Body, nil
}
