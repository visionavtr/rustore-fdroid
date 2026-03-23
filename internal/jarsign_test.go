package internal

import (
	"archive/zip"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
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
