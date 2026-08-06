package internal

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.mozilla.org/pkcs7"
)

func SignRepository(repoPath, certPath, keyPath string) (string, error) {
	indexData, err := os.ReadFile(IndexV1Path(repoPath))
	if err != nil {
		return "", fmt.Errorf("read index: %w", err)
	}
	idx, err := LoadIndexV1(repoPath)
	if err != nil {
		return "", err
	}
	indexV2Data, err := BuildIndexV2(repoPath, idx)
	if err != nil {
		return "", fmt.Errorf("build index-v2: %w", err)
	}
	entryData, err := BuildEntry(idx, indexV2Data, IndexV2PackageCount(idx))
	if err != nil {
		return "", fmt.Errorf("build entry: %w", err)
	}

	cert, key, err := loadCertAndKey(certPath, keyPath)
	if err != nil {
		return "", err
	}
	indexJar, err := createSignedJAR("index-v1.json", indexData, cert, key, false)
	if err != nil {
		return "", fmt.Errorf("sign index-v1: %w", err)
	}
	entryJar, err := createSignedJAR("entry.json", entryData, cert, key, true)
	if err != nil {
		return "", fmt.Errorf("sign entry: %w", err)
	}

	// Stage every file before replacing any published index. The signed entry is
	// the v2 trust anchor, so commit it last.
	files := []struct {
		path string
		data []byte
	}{
		{filepath.Join(repoPath, "index-v1.jar"), indexJar},
		{IndexV2Path(repoPath), indexV2Data},
		{EntryPath(repoPath), entryData},
		{filepath.Join(repoPath, "entry.jar"), entryJar},
	}
	staged := make([]string, len(files))
	defer func() {
		for _, path := range staged {
			if path != "" {
				_ = os.Remove(path)
			}
		}
	}()
	for i, file := range files {
		if info, err := os.Lstat(file.path); err == nil && info.IsDir() {
			return "", fmt.Errorf("publish %s: destination is a directory", filepath.Base(file.path))
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("check %s: %w", filepath.Base(file.path), err)
		}
		staged[i], err = stageFile(file.path, file.data, 0o644)
		if err != nil {
			return "", fmt.Errorf("stage %s: %w", filepath.Base(file.path), err)
		}
	}
	for i, file := range files {
		if err := os.Rename(staged[i], file.path); err != nil {
			return "", fmt.Errorf("publish %s: %w", filepath.Base(file.path), err)
		}
		staged[i] = ""
	}
	dir, err := os.Open(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repository directory: %w", err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return "", fmt.Errorf("sync repository directory: %w", err)
	}
	if err := dir.Close(); err != nil {
		return "", fmt.Errorf("close repository directory: %w", err)
	}

	fingerprint := sha256.Sum256(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(fingerprint[:])), nil
}

func createSignedJAR(dataName string, data []byte, cert *x509.Certificate, key crypto.PrivateKey, modern bool) ([]byte, error) {
	manifest := buildManifestFor(dataName, data)
	sf := buildSignatureFileFor(dataName, manifest)
	sigData, err := createPKCS7SignatureWithDigest(sf, cert, key, modern)
	if err != nil {
		return nil, err
	}
	ext, err := keyTypeExtension(key)
	if err != nil {
		return nil, err
	}

	entries := []struct {
		name string
		data []byte
	}{
		{"META-INF/MANIFEST.MF", manifest},
		{"META-INF/CERT.SF", sf},
		{"META-INF/CERT." + ext, sigData},
		{dataName, data},
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e.name)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %s: %w", e.name, err)
		}
		if _, err := fw.Write(e.data); err != nil {
			return nil, fmt.Errorf("write zip entry %s: %w", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close jar: %w", err)
	}
	return buf.Bytes(), nil
}

func buildManifestFor(dataName string, data []byte) []byte {
	digest := sha256.Sum256(data)
	b64 := base64.StdEncoding.EncodeToString(digest[:])

	return []byte("Manifest-Version: 1.0\r\n\r\n" +
		"Name: " + dataName + "\r\n" +
		"SHA-256-Digest: " + b64 + "\r\n\r\n")
}

func buildSignatureFileFor(dataName string, manifest []byte) []byte {
	manifestDigest := sha256.Sum256(manifest)
	manifestB64 := base64.StdEncoding.EncodeToString(manifestDigest[:])

	// Digest of the individual section (everything after the main section)
	section := findSection(manifest)
	sectionDigest := sha256.Sum256(section)
	sectionB64 := base64.StdEncoding.EncodeToString(sectionDigest[:])

	return []byte("Signature-Version: 1.0\r\n" +
		"SHA-256-Digest-Manifest: " + manifestB64 + "\r\n\r\n" +
		"Name: " + dataName + "\r\n" +
		"SHA-256-Digest: " + sectionB64 + "\r\n\r\n")
}

func findSection(manifest []byte) []byte {
	// Find the second section (after the main "Manifest-Version" section)
	// Sections are separated by \r\n\r\n
	i := 0
	for i < len(manifest) {
		if i+3 < len(manifest) && manifest[i] == '\r' && manifest[i+1] == '\n' && manifest[i+2] == '\r' && manifest[i+3] == '\n' {
			return manifest[i+2:] // skip first \r\n, return from second \r\n onwards
		}
		i++
	}
	return manifest
}

func loadCertAndKey(certPath, keyPath string) (*x509.Certificate, crypto.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read cert: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse cert: %w", err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode key PEM")
	}

	var key crypto.PrivateKey
	// Try PKCS8 first, then PKCS1
	key, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			key, err = x509.ParseECPrivateKey(keyBlock.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("parse private key: unsupported key format")
			}
		}
	}

	return cert, key, nil
}

func createPKCS7SignatureWithDigest(data []byte, cert *x509.Certificate, key crypto.PrivateKey, modern bool) ([]byte, error) {
	signedData, err := pkcs7.NewSignedData(data)
	if err != nil {
		return nil, fmt.Errorf("create signed data: %w", err)
	}
	if modern {
		signedData.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	}

	if err := signedData.AddSigner(cert, key, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("add signer: %w", err)
	}

	result, err := signedData.Finish()
	if err != nil {
		return nil, fmt.Errorf("finish signature: %w", err)
	}

	return result, nil
}

func keyTypeExtension(key crypto.PrivateKey) (string, error) {
	switch key.(type) {
	case *rsa.PrivateKey:
		return "RSA", nil
	case *ecdsa.PrivateKey:
		return "EC", nil
	default:
		return "", fmt.Errorf("unsupported key type %T", key)
	}
}
