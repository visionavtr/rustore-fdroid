package internal

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildManifest_UsesSHA256(t *testing.T) {
	data := []byte(`{"repo":{"name":"test"}}`)
	manifest := buildManifest(data)
	s := string(manifest)

	if strings.Contains(s, "SHA1-Digest") {
		t.Error("manifest still contains SHA1-Digest")
	}
	if !strings.Contains(s, "SHA-256-Digest:") {
		t.Error("manifest missing SHA-256-Digest")
	}

	// Verify the digest is correct
	digest := sha256.Sum256(data)
	expected := base64.StdEncoding.EncodeToString(digest[:])
	if !strings.Contains(s, expected) {
		t.Errorf("manifest digest mismatch, want %s in:\n%s", expected, s)
	}
}

func TestBuildSignatureFile_UsesSHA256(t *testing.T) {
	manifest := buildManifest([]byte(`{"test":true}`))
	sf := buildSignatureFile(manifest)
	s := string(sf)

	if strings.Contains(s, "SHA1-") {
		t.Error("signature file still contains SHA1 references")
	}
	if !strings.Contains(s, "SHA-256-Digest-Manifest:") {
		t.Error("signature file missing SHA-256-Digest-Manifest")
	}
	if !strings.Contains(s, "SHA-256-Digest:") {
		t.Error("signature file missing SHA-256-Digest for section")
	}
}

func TestKeyTypeExtension(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, ed25519Key, _ := ed25519.GenerateKey(rand.Reader)

	tests := []struct {
		name    string
		key     interface{}
		want    string
		wantErr bool
	}{
		{"RSA key", rsaKey, "RSA", false},
		{"ECDSA key", ecKey, "EC", false},
		{"Ed25519 key (unsupported)", ed25519Key, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := keyTypeExtension(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("keyTypeExtension() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("keyTypeExtension() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSignJAR_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal index
	indexData := []byte(`{"repo":{"name":"test","timestamp":0,"version":0},"apps":[],"packages":{}}`)
	if err := os.WriteFile(filepath.Join(dir, "index-v1.json"), indexData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Generate a self-signed cert + key
	certPath, keyPath := generateTestCert(t, dir)

	if err := SignJAR(dir, certPath, keyPath); err != nil {
		t.Fatalf("SignJAR: %v", err)
	}
	for _, name := range []string{"index-v2.json", "entry.json", "entry.jar"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s not created: %v", name, err)
		}
	}

	jarPath := filepath.Join(dir, "index-v1.jar")
	if _, err := os.Stat(jarPath); err != nil {
		t.Fatalf("JAR not created: %v", err)
	}

	// Open the JAR and verify contents
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		t.Fatalf("open JAR: %v", err)
	}
	defer r.Close()

	wantEntries := map[string]bool{
		"META-INF/MANIFEST.MF": false,
		"META-INF/CERT.SF":     false,
		"META-INF/CERT.RSA":    false,
		"index-v1.json":        false,
	}

	for _, f := range r.File {
		if _, ok := wantEntries[f.Name]; ok {
			wantEntries[f.Name] = true
		}
	}

	for name, found := range wantEntries {
		if !found {
			t.Errorf("missing JAR entry: %s", name)
		}
	}

	entryData, err := os.ReadFile(filepath.Join(dir, "entry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entry Entry
	if err := json.Unmarshal(entryData, &entry); err != nil {
		t.Fatalf("parse entry.json: %v", err)
	}
	indexV2Data, err := os.ReadFile(filepath.Join(dir, "index-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	indexHash := sha256.Sum256(indexV2Data)
	if entry.Index.SHA256 != hex.EncodeToString(indexHash[:]) {
		t.Fatalf("entry index hash = %q, want %x", entry.Index.SHA256, indexHash)
	}
	if entry.Version != MetadataVersion || entry.Index.Name != "/index-v2.json" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	assertJARData(t, filepath.Join(dir, "entry.jar"), "entry.json", entryData)
}

func assertJARData(t *testing.T, jarPath, dataName string, want []byte) {
	t.Helper()
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		t.Fatalf("open %s: %v", jarPath, err)
	}
	defer r.Close()
	for _, file := range r.File {
		if file.Name != dataName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s in JAR differs from file", dataName)
		}
		return
	}
	t.Fatalf("missing %s in %s", dataName, jarPath)
}

func generateTestCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	certFile, _ := os.Create(certPath)
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certFile.Close()

	keyPath = filepath.Join(dir, "key.pem")
	keyFile, _ := os.Create(keyPath)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	keyFile.Close()

	return certPath, keyPath
}
