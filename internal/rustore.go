package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	rustoreBaseURL    = "https://backapi.rustore.ru/applicationData"
	overallInfoURL    = rustoreBaseURL + "/overallInfo/"
	downloadLinkURL   = rustoreBaseURL + "/v2/download-link"
)

type OverallInfoResponse struct {
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

func FetchAppInfo(packageID string) (*AppInfo, error) {
	resp, err := http.Get(overallInfoURL + packageID)
	if err != nil {
		return nil, fmt.Errorf("fetch app info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result OverallInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse app info: %w", err)
	}

	return &result.Body, nil
}

func FetchDownloadLink(appID int) (*DownloadBody, error) {
	payload, _ := json.Marshal(map[string]int{"appId": appID})

	resp, err := http.Post(downloadLinkURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("fetch download link: %w", err)
	}
	defer resp.Body.Close()

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
