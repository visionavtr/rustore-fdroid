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
		resp := OverallInfoResponse{
			rustoreResponse: rustoreResponse{Code: "OK"},
			Body: AppInfo{
				AppID:       12345,
				PackageName: "ru.sberbankmobile",
				AppName:     "Sberbank",
				VersionCode: 100,
				VersionName: "1.0.0",
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
