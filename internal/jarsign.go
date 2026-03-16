package internal

import (
	"archive/zip"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"go.mozilla.org/pkcs7"
)

func SignJAR(repoPath, certPath, keyPath string) error {
	indexData, err := os.ReadFile(IndexV1Path(repoPath))
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}

	manifest := buildManifest(indexData)
	sf := buildSignatureFile(manifest)

	cert, key, err := loadCertAndKey(certPath, keyPath)
	if err != nil {
		return err
	}

	sigData, err := createPKCS7Signature(sf, cert, key)
	if err != nil {
		return err
	}

	ext := keyTypeExtension(key)

	jarPath := IndexV1Path(repoPath)[:len(IndexV1Path(repoPath))-5] + ".jar"

	f, err := os.Create(jarPath)
	if err != nil {
		return fmt.Errorf("create jar: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	entries := []struct {
		name string
		data []byte
	}{
		{"META-INF/MANIFEST.MF", manifest},
		{"META-INF/CERT.SF", sf},
		{"META-INF/CERT." + ext, sigData},
		{"index-v1.json", indexData},
	}

	for _, e := range entries {
		fw, err := w.Create(e.name)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", e.name, err)
		}
		if _, err := fw.Write(e.data); err != nil {
			return fmt.Errorf("write zip entry %s: %w", e.name, err)
		}
	}

	return nil
}

func buildManifest(indexData []byte) []byte {
	digest := sha1.Sum(indexData)
	b64 := base64.StdEncoding.EncodeToString(digest[:])

	return []byte("Manifest-Version: 1.0\r\n\r\n" +
		"Name: index-v1.json\r\n" +
		"SHA1-Digest: " + b64 + "\r\n\r\n")
}

func buildSignatureFile(manifest []byte) []byte {
	manifestDigest := sha1.Sum(manifest)
	manifestB64 := base64.StdEncoding.EncodeToString(manifestDigest[:])

	// Digest of the individual section (everything after the main section)
	section := findSection(manifest)
	sectionDigest := sha1.Sum(section)
	sectionB64 := base64.StdEncoding.EncodeToString(sectionDigest[:])

	return []byte("Signature-Version: 1.0\r\n" +
		"SHA1-Digest-Manifest: " + manifestB64 + "\r\n\r\n" +
		"Name: index-v1.json\r\n" +
		"SHA1-Digest: " + sectionB64 + "\r\n\r\n")
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

func createPKCS7Signature(data []byte, cert *x509.Certificate, key crypto.PrivateKey) ([]byte, error) {
	signedData, err := pkcs7.NewSignedData(data)
	if err != nil {
		return nil, fmt.Errorf("create signed data: %w", err)
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

func keyTypeExtension(key crypto.PrivateKey) string {
	switch key.(type) {
	case *rsa.PrivateKey:
		return "RSA"
	case *ecdsa.PrivateKey:
		return "EC"
	default:
		return "RSA"
	}
}
