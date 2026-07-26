package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadAndGetSHA256PublishesAtomically(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "app.apk")
	if err := os.WriteFile(output, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failing.Close()
	if _, _, err := DownloadAndGetSHA256(failing.URL, output, 0); err == nil {
		t.Fatal("expected download error")
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("failed download replaced destination with %q", data)
	}

	payload := []byte("new APK payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	if _, _, err := DownloadAndGetSHA256(server.URL, output, int64(len(payload)+1)); err == nil {
		t.Fatal("expected size mismatch")
	}
	data, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("short download replaced destination with %q", data)
	}
	_, gotHash, err := DownloadAndGetSHA256(server.URL, output, int64(len(payload)))
	if err != nil {
		t.Fatalf("DownloadAndGetSHA256: %v", err)
	}
	wantHash := sha256.Sum256(payload)
	if gotHash != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("hash = %s, want %x", gotHash, wantHash)
	}
	data, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("destination = %q", data)
	}
}
