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

// ExtractAPKSigners returns the SHA-256 fingerprints of all APK signing
// certificates found in the v1 or v2 signature data.
func ExtractAPKSigners(apkPath string) ([]string, error) {
	signers, err := extractV1Signers(apkPath)
	if err == nil && len(signers) > 0 {
		return signers, nil
	}
	return extractV2Signers(apkPath)
}

// extractV1Signers reads JAR-style PKCS7 signatures from META-INF/*.RSA|DSA|EC.
func extractV1Signers(apkPath string) ([]string, error) {
	r, err := zip.OpenReader(apkPath)
	if err != nil {
		return nil, fmt.Errorf("open APK: %w", err)
	}
	defer r.Close()

	var signers []string
	seen := make(map[string]struct{})
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
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}

		buf, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}

		p7, err := pkcs7.Parse(buf)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS7 from %s: %w", f.Name, err)
		}

		cert := p7.GetOnlySigner()
		if cert == nil && len(p7.Certificates) > 0 {
			cert = p7.Certificates[0]
		}
		if cert == nil {
			continue
		}
		hash := sha256.Sum256(cert.Raw)
		fingerprint := hex.EncodeToString(hash[:])
		if _, ok := seen[fingerprint]; !ok {
			seen[fingerprint] = struct{}{}
			signers = append(signers, fingerprint)
		}
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no v1 signing certificate found")
	}
	return signers, nil
}

// extractV2Signers reads signing certificates from the APK Signature Scheme v2
// block embedded in the APK binary.
func extractV2Signers(apkPath string) ([]string, error) {
	f, err := os.Open(apkPath)
	if err != nil {
		return nil, fmt.Errorf("open APK: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Find End of Central Directory (last 22 bytes for no-comment case)
	if fi.Size() < 22 {
		return nil, fmt.Errorf("file too small to be an APK")
	}

	eocd := make([]byte, 22)
	if _, err := f.ReadAt(eocd, fi.Size()-22); err != nil {
		return nil, fmt.Errorf("read EOCD: %w", err)
	}
	if binary.LittleEndian.Uint32(eocd[:4]) != 0x06054b50 {
		return nil, fmt.Errorf("EOCD signature not found")
	}

	cdOffset := int64(binary.LittleEndian.Uint32(eocd[16:20]))

	// APK Signing Block sits just before the Central Directory.
	// Its last 24 bytes: 8-byte block size + 16-byte magic.
	if cdOffset < 24 {
		return nil, fmt.Errorf("no space for APK Signing Block")
	}

	tail := make([]byte, 24)
	if _, err := f.ReadAt(tail, cdOffset-24); err != nil {
		return nil, fmt.Errorf("read signing block tail: %w", err)
	}

	if string(tail[8:]) != apkSigBlockMagic {
		return nil, fmt.Errorf("APK Signing Block magic not found")
	}

	blockSize := int64(binary.LittleEndian.Uint64(tail[:8]))
	blockStart := cdOffset - blockSize - 8
	if blockSize < 24 || blockStart < 0 || blockSize > fi.Size() {
		return nil, fmt.Errorf("invalid APK Signing Block size")
	}

	block := make([]byte, blockSize+8)
	if _, err := f.ReadAt(block, blockStart); err != nil {
		return nil, fmt.Errorf("read signing block: %w", err)
	}

	// Parse ID-value pairs starting after the 8-byte size header,
	// stopping before the 24-byte tail.
	offset := 8
	end := len(block) - 24
	for offset < end {
		if offset+12 > end {
			break
		}
		pairSize64 := binary.LittleEndian.Uint64(block[offset : offset+8])
		if pairSize64 < 4 || pairSize64 > uint64(end-offset-8) {
			return nil, fmt.Errorf("invalid APK Signing Block entry size")
		}
		pairSize := int(pairSize64)
		pairID := binary.LittleEndian.Uint32(block[offset+8 : offset+12])

		if pairID == apkSigV2BlockID {
			return parseV2Signers(block[offset+12 : offset+8+pairSize])
		}
		offset += 8 + pairSize
	}

	return nil, fmt.Errorf("APK Signature Scheme v2 block not found")
}

// parseV2Signers extracts the first certificate from each v2 signer.
// Format: length-prefixed sequence of signers, each containing
// signed_data (digests, certificates, ...), signatures, public_key.
func parseV2Signers(data []byte) ([]string, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("v2 signers block too short")
	}
	signersLength := int(binary.LittleEndian.Uint32(data[:4]))
	if signersLength > len(data)-4 {
		return nil, fmt.Errorf("v2 signers block truncated")
	}

	off := 4
	end := off + signersLength
	var fingerprints []string
	for off < end {
		signer, err := readLengthPrefixed(data[:end], &off)
		if err != nil {
			return nil, fmt.Errorf("read v2 signer: %w", err)
		}
		certDER, err := firstV2SignerCertificate(signer)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(certDER)
		fingerprints = append(fingerprints, hex.EncodeToString(hash[:]))
	}
	if len(fingerprints) == 0 {
		return nil, fmt.Errorf("no v2 signing certificates found")
	}
	return fingerprints, nil
}

func firstV2SignerCertificate(signer []byte) ([]byte, error) {
	off := 0
	signedData, err := readLengthPrefixed(signer, &off)
	if err != nil {
		return nil, fmt.Errorf("read v2 signed data: %w", err)
	}
	// signed_data: digests (skip), then certificates
	sd := 0
	if _, err := readLengthPrefixed(signedData, &sd); err != nil {
		return nil, fmt.Errorf("read v2 digests: %w", err)
	}
	certificates, err := readLengthPrefixed(signedData, &sd)
	if err != nil {
		return nil, fmt.Errorf("read v2 certificates: %w", err)
	}
	certOffset := 0
	certDER, err := readLengthPrefixed(certificates, &certOffset)
	if err != nil {
		return nil, fmt.Errorf("read v2 certificate: %w", err)
	}
	return certDER, nil
}

func readLengthPrefixed(data []byte, offset *int) ([]byte, error) {
	if *offset < 0 || *offset+4 > len(data) {
		return nil, fmt.Errorf("length prefix truncated")
	}
	length := int(binary.LittleEndian.Uint32(data[*offset : *offset+4]))
	*offset += 4
	if length < 0 || length > len(data)-*offset {
		return nil, fmt.Errorf("length-prefixed value truncated")
	}
	value := data[*offset : *offset+length]
	*offset += length
	return value, nil
}
