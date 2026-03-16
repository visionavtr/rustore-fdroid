package internal

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.mozilla.org/pkcs7"
)

const apkSigBlockMagic = "APK Sig Block 42"
const apkSigV2BlockID = 0x7109871a

// ExtractAPKSig extracts the hex-encoded SHA-256 fingerprint of the APK
// signing certificate. It tries JAR signature (v1) first, then falls back
// to APK Signature Scheme v2.
func ExtractAPKSig(apkPath string) (string, error) {
	sig, err := extractV1Sig(apkPath)
	if err == nil {
		return sig, nil
	}
	return extractV2Sig(apkPath)
}

// extractV1Sig reads the JAR-style PKCS7 signature from META-INF/*.RSA|DSA|EC.
func extractV1Sig(apkPath string) (string, error) {
	r, err := zip.OpenReader(apkPath)
	if err != nil {
		return "", fmt.Errorf("open APK: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		dir := filepath.Dir(f.Name)
		ext := strings.ToUpper(filepath.Ext(f.Name))
		if !strings.EqualFold(dir, "META-INF") {
			continue
		}
		if ext != ".RSA" && ext != ".DSA" && ext != ".EC" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open %s: %w", f.Name, err)
		}
		defer rc.Close()

		buf, err := io.ReadAll(rc)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", f.Name, err)
		}

		p7, err := pkcs7.Parse(buf)
		if err != nil {
			return "", fmt.Errorf("parse PKCS7 from %s: %w", f.Name, err)
		}

		if len(p7.Certificates) == 0 {
			return "", fmt.Errorf("no certificates in %s", f.Name)
		}

		hash := sha256.Sum256(p7.Certificates[0].Raw)
		return hex.EncodeToString(hash[:]), nil
	}

	return "", fmt.Errorf("no v1 signing certificate found")
}

// extractV2Sig reads the signing certificate from the APK Signature Scheme v2
// block embedded in the APK binary.
func extractV2Sig(apkPath string) (string, error) {
	f, err := os.Open(apkPath)
	if err != nil {
		return "", fmt.Errorf("open APK: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	// Find End of Central Directory (last 22 bytes for no-comment case)
	if fi.Size() < 22 {
		return "", fmt.Errorf("file too small to be an APK")
	}

	eocd := make([]byte, 22)
	if _, err := f.ReadAt(eocd, fi.Size()-22); err != nil {
		return "", fmt.Errorf("read EOCD: %w", err)
	}
	if binary.LittleEndian.Uint32(eocd[:4]) != 0x06054b50 {
		return "", fmt.Errorf("EOCD signature not found")
	}

	cdOffset := int64(binary.LittleEndian.Uint32(eocd[16:20]))

	// APK Signing Block sits just before the Central Directory.
	// Its last 24 bytes: 8-byte block size + 16-byte magic.
	if cdOffset < 24 {
		return "", fmt.Errorf("no space for APK Signing Block")
	}

	tail := make([]byte, 24)
	if _, err := f.ReadAt(tail, cdOffset-24); err != nil {
		return "", fmt.Errorf("read signing block tail: %w", err)
	}

	if string(tail[8:]) != apkSigBlockMagic {
		return "", fmt.Errorf("APK Signing Block magic not found")
	}

	blockSize := int64(binary.LittleEndian.Uint64(tail[:8]))
	blockStart := cdOffset - blockSize - 8

	block := make([]byte, blockSize+8)
	if _, err := f.ReadAt(block, blockStart); err != nil {
		return "", fmt.Errorf("read signing block: %w", err)
	}

	// Parse ID-value pairs starting after the 8-byte size header,
	// stopping before the 24-byte tail.
	offset := 8
	end := len(block) - 24
	for offset < end {
		if offset+12 > end {
			break
		}
		pairSize := int(binary.LittleEndian.Uint64(block[offset : offset+8]))
		pairID := binary.LittleEndian.Uint32(block[offset+8 : offset+12])

		if pairID == apkSigV2BlockID {
			return parseV2Signers(block[offset+12 : offset+8+pairSize])
		}
		offset += 8 + pairSize
	}

	return "", fmt.Errorf("APK Signature Scheme v2 block not found")
}

// parseV2Signers extracts the first certificate from a v2 signers block.
// Format: length-prefixed sequence of signers, each containing
// signed_data (digests, certificates, ...), signatures, public_key.
func parseV2Signers(data []byte) (string, error) {
	if len(data) < 4 {
		return "", fmt.Errorf("v2 signers block too short")
	}

	// Skip signers sequence length prefix
	off := 4

	// First signer length
	if off+4 > len(data) {
		return "", fmt.Errorf("v2 signer truncated")
	}
	signerLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4
	if off+signerLen > len(data) {
		return "", fmt.Errorf("v2 signer data truncated")
	}
	signer := data[off : off+signerLen]

	// signed_data is the first length-prefixed field in the signer
	if len(signer) < 4 {
		return "", fmt.Errorf("signed_data truncated")
	}
	signedDataLen := int(binary.LittleEndian.Uint32(signer[:4]))
	signedData := signer[4 : 4+signedDataLen]

	// signed_data: digests (skip), then certificates
	sd := 0
	if sd+4 > len(signedData) {
		return "", fmt.Errorf("digests length truncated")
	}
	digestsLen := int(binary.LittleEndian.Uint32(signedData[sd : sd+4]))
	sd += 4 + digestsLen

	// certificates: length-prefixed sequence
	if sd+4 > len(signedData) {
		return "", fmt.Errorf("certificates length truncated")
	}
	sd += 4 // skip certificates sequence length

	// First certificate
	if sd+4 > len(signedData) {
		return "", fmt.Errorf("certificate length truncated")
	}
	certLen := int(binary.LittleEndian.Uint32(signedData[sd : sd+4]))
	sd += 4
	if sd+certLen > len(signedData) {
		return "", fmt.Errorf("certificate data truncated")
	}
	certDER := signedData[sd : sd+certLen]

	hash := sha256.Sum256(certDER)
	return hex.EncodeToString(hash[:]), nil
}
