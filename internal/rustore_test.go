package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func init() {
	// Disable retry backoff in tests.
	retryBaseBackoff = time.Millisecond
}

func TestValidatePackageName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid sberbank", "ru.sberbankmobile", false},
		{"valid autodoc", "ru.autodoc.autodocapp", false},
		{"valid deep nesting", "com.example.app.feature.v2", false},
		{"valid with underscore", "com.example.my_app", false},
		{"empty", "", true},
		{"single segment", "sberbank", true},
		{"path traversal dots", "../etc/passwd", true},
		{"path traversal embedded", "ru.sberbank/../../../etc", true},
		{"slash in name", "ru.sberbank/evil", true},
		{"backslash in name", "ru.sberbank\\evil", true},
		{"starts with dot", ".ru.sberbank", true},
		{"segment starts with digit", "ru.1bank", true},
		{"has spaces", "ru.sber bank", true},
		{"has special chars", "ru.sber$bank", true},
		{"single dot", ".", true},
		{"double dot", "..", true},
		{"trailing dot", "ru.sberbank.", true},
		{"leading dot", ".sberbank.ru", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePackageName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePackageName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestFetchAppInfo_HTTPStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	// Temporarily override the client to use the test server
	origGet := fetchAppInfoURL
	fetchAppInfoURL = ts.URL + "/"
	defer func() { fetchAppInfoURL = origGet }()

	_, err := FetchAppInfo("ru.sberbankmobile")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestFetchAppInfo_InvalidPackageName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OverallInfoResponse{
			rustoreResponse: rustoreResponse{Code: "OK"},
			Body:            AppInfo{PackageName: "../evil"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	origGet := fetchAppInfoURL
	fetchAppInfoURL = ts.URL + "/"
	defer func() { fetchAppInfoURL = origGet }()

	_, err := FetchAppInfo("ru.sberbankmobile")
	if err == nil {
		t.Fatal("expected error for invalid package name, got nil")
	}
}

func TestFetchAppInfo_ValidResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(rustoreVersionHeader); got != rustoreVersionCode {
			t.Errorf("%s header = %q, want %q", rustoreVersionHeader, got, rustoreVersionCode)
		}
		resp := OverallInfoResponse{
			rustoreResponse: rustoreResponse{Code: "OK"},
			Body: AppInfo{
				AppID:         12345,
				PackageName:   "ru.sberbankmobile",
				AppName:       "Sberbank",
				WhatsNew:      "Changes",
				VersionCode:   100,
				VersionName:   "1.0.0",
				MaxSdkVersion: 35,
				FileURLs: []AppFile{{
					URL:     "https://example.com/screenshot.png",
					Ordinal: 1,
					Type:    "SCREENSHOT",
				}},
				DeveloperContacts: DeveloperContacts{Email: "dev@example.com"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	origGet := fetchAppInfoURL
	fetchAppInfoURL = ts.URL + "/"
	defer func() { fetchAppInfoURL = origGet }()

	info, err := FetchAppInfo("ru.sberbankmobile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PackageName != "ru.sberbankmobile" {
		t.Errorf("got PackageName=%q, want %q", info.PackageName, "ru.sberbankmobile")
	}
	if info.AppID != 12345 {
		t.Errorf("got AppID=%d, want 12345", info.AppID)
	}
	if info.WhatsNew != "Changes" || info.MaxSdkVersion != 35 || info.DeveloperContacts.Email != "dev@example.com" {
		t.Errorf("rich metadata was not decoded: %+v", info)
	}
	if len(info.FileURLs) != 1 || info.FileURLs[0].Type != "SCREENSHOT" {
		t.Errorf("file URLs were not decoded: %+v", info.FileURLs)
	}
}

func TestFetchAppInfo_AppNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OverallInfoResponse{
			rustoreResponse: rustoreResponse{Code: "ERROR", Message: "not found"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	origGet := fetchAppInfoURL
	fetchAppInfoURL = ts.URL + "/"
	defer func() { fetchAppInfoURL = origGet }()

	_, err := FetchAppInfo("com.nonexistent.app")
	if err == nil {
		t.Fatal("expected error for ERROR code, got nil")
	}
}

func TestFetchDownloadLink_HTTPStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	origURL := fetchDownloadLinkURL
	fetchDownloadLinkURL = ts.URL
	defer func() { fetchDownloadLinkURL = origURL }()

	_, err := FetchDownloadLink(12345)
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

func TestFetchDownloadLink_RequiredHeadersAndDirectAPK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get(rustoreVersionHeader); got != rustoreVersionCode {
			t.Errorf("%s header = %q, want %q", rustoreVersionHeader, got, rustoreVersionCode)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var request struct {
			AppID        int  `json:"appId"`
			FirstInstall bool `json:"firstInstall"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if request.AppID != 12345 || !request.FirstInstall {
			t.Errorf("unexpected request body: %+v", request)
		}

		resp := DownloadLinkResponse{
			rustoreResponse: rustoreResponse{Code: "OK"},
			Body: DownloadBody{
				DownloadURLs: []DownloadURL{{
					URL:  "https://static.rustore.ru/app.ZIP?token=test",
					Size: 100,
					Hash: "abcdef",
				}},
				Signature: "signature",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer ts.Close()

	origURL := fetchDownloadLinkURL
	fetchDownloadLinkURL = ts.URL
	defer func() { fetchDownloadLinkURL = origURL }()

	result, err := FetchDownloadLink(12345)
	if err != nil {
		t.Fatalf("FetchDownloadLink: %v", err)
	}
	if len(result.DownloadURLs) != 1 {
		t.Fatalf("download URLs = %d, want 1", len(result.DownloadURLs))
	}
	download := result.DownloadURLs[0]
	if download.URL != "https://static.rustore.ru/app.apk?token=test" {
		t.Errorf("URL = %q, want direct APK URL", download.URL)
	}
	if download.Size != 0 || download.Hash != "" {
		t.Errorf("ZIP metadata was not cleared: %+v", download)
	}
	if result.Signature != "signature" {
		t.Errorf("signature = %q, want signature", result.Signature)
	}
}

func TestNormalizeAPKDownload_LeavesDirectAPKMetadata(t *testing.T) {
	download := DownloadURL{
		URL:  "https://static.rustore.ru/app.apk",
		Size: 100,
		Hash: "abcdef",
	}

	normalizeAPKDownload(&download)

	if download.URL != "https://static.rustore.ru/app.apk" || download.Size != 100 || download.Hash != "abcdef" {
		t.Errorf("direct APK metadata changed: %+v", download)
	}
}
