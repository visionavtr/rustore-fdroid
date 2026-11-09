package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestSigningPair(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "repo.crt")
	keyPath = filepath.Join(dir, "repo.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func runRoot(t *testing.T, args ...string) error {
	t.Helper()
	originalRepoPath := repoPath
	t.Cleanup(func() {
		repoPath = originalRepoPath
		rootCmd.SetArgs(nil)
		rootCmd.SilenceUsage = false
	})
	rootCmd.SetArgs(args)
	rootCmd.SilenceUsage = true
	return rootCmd.Execute()
}

func TestSyncSignsUpdatedRepository(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	certPath, keyPath := writeTestSigningPair(t, dir)

	if err := runRoot(t, "-r", repo, "init", "-n", "Test", "-d", "Test repo", "-a", "https://example.com/repo"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := runRoot(t, "-r", repo, "sync", "-c", certPath, "-k", keyPath); err != nil {
		t.Fatalf("sync: %v", err)
	}

	for _, name := range []string{"index-v1.jar", "index-v2.json", "entry.json", "entry.jar"} {
		if _, err := os.Stat(filepath.Join(repo, name)); err != nil {
			t.Errorf("sync did not publish %s: %v", name, err)
		}
	}
}

func TestSyncDoesNotSignWhenUpdateFails(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	certPath, keyPath := writeTestSigningPair(t, dir)

	if err := runRoot(t, "-r", repo, "sync", "-c", certPath, "-k", keyPath); err == nil {
		t.Fatal("sync succeeded without an index")
	}
	if _, err := os.Stat(filepath.Join(repo, "index-v1.jar")); !os.IsNotExist(err) {
		t.Fatalf("index-v1.jar was published after a failed update: %v", err)
	}
}
