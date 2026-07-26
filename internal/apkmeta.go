package internal

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"
)

const CurrentAPKMetadataVersion = 1

type APKMetadata struct {
	PackageName         string
	VersionCode         int
	VersionName         string
	MinSDKVersion       int
	TargetSDKVersion    int
	MaxSDKVersion       int
	NativeCode          []string
	Features            []string
	UsesPermission      [][]any
	UsesPermissionSDK23 [][]any
	Signers             []string
	HasMultipleSigners  bool
}

type manifestFeature struct {
	Name     androidbinary.String `xml:"http://schemas.android.com/apk/res/android name,attr"`
	Required androidbinary.String `xml:"http://schemas.android.com/apk/res/android required,attr"`
}

type manifestMetadata struct {
	Package              androidbinary.String `xml:"package,attr"`
	VersionCode          androidbinary.Int32  `xml:"http://schemas.android.com/apk/res/android versionCode,attr"`
	VersionName          androidbinary.String `xml:"http://schemas.android.com/apk/res/android versionName,attr"`
	SDK                  apk.UsesSDK          `xml:"uses-sdk"`
	UsesPermissions      []apk.UsesPermission `xml:"uses-permission"`
	UsesPermissionsSDK23 []apk.UsesPermission `xml:"uses-permission-sdk-23"`
	UsesFeatures         []manifestFeature    `xml:"uses-feature"`
}

func ExtractAPKMetadata(apkPath string) (*APKMetadata, error) {
	r, err := zip.OpenReader(apkPath)
	if err != nil {
		return nil, fmt.Errorf("open APK: %w", err)
	}
	defer r.Close()

	var manifestData []byte
	nativeCode := make(map[string]struct{})
	for _, file := range r.File {
		if file.Name == "AndroidManifest.xml" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("open AndroidManifest.xml: %w", err)
			}
			manifestData, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, fmt.Errorf("read AndroidManifest.xml: %w", err)
			}
		}
		parts := strings.Split(file.Name, "/")
		if len(parts) >= 3 && parts[0] == "lib" && parts[1] != "" && strings.HasSuffix(parts[len(parts)-1], ".so") {
			nativeCode[parts[1]] = struct{}{}
		}
	}
	if len(manifestData) == 0 {
		return nil, fmt.Errorf("AndroidManifest.xml not found")
	}

	xmlFile, err := androidbinary.NewXMLFile(bytes.NewReader(manifestData))
	if err != nil {
		return nil, fmt.Errorf("parse AndroidManifest.xml: %w", err)
	}
	var manifest manifestMetadata
	if err := xmlFile.Decode(&manifest, nil, nil); err != nil {
		return nil, fmt.Errorf("decode AndroidManifest.xml: %w", err)
	}

	packageName, _ := manifest.Package.String()
	versionName, _ := manifest.VersionName.String()
	metadata := &APKMetadata{
		PackageName:         packageName,
		VersionCode:         manifestInt(manifest.VersionCode),
		VersionName:         versionName,
		MinSDKVersion:       manifestInt(manifest.SDK.Min),
		TargetSDKVersion:    manifestInt(manifest.SDK.Target),
		MaxSDKVersion:       manifestInt(manifest.SDK.Max),
		NativeCode:          mapKeys(nativeCode),
		UsesPermission:      manifestPermissions(manifest.UsesPermissions),
		UsesPermissionSDK23: manifestPermissions(manifest.UsesPermissionsSDK23),
	}
	for _, feature := range manifest.UsesFeatures {
		name, err := feature.Name.String()
		if err != nil || name == "" {
			continue
		}
		required, _ := feature.Required.String()
		if required != "" {
			isRequired, err := strconv.ParseBool(required)
			if err == nil && !isRequired {
				continue
			}
		}
		metadata.Features = append(metadata.Features, name)
	}
	sort.Strings(metadata.Features)

	metadata.Signers, err = ExtractAPKSigners(apkPath)
	if err != nil {
		return nil, err
	}
	metadata.HasMultipleSigners = len(metadata.Signers) > 1
	return metadata, nil
}

func manifestInt(value androidbinary.Int32) int {
	result, err := value.Int32()
	if err != nil || result <= 0 {
		return 0
	}
	return int(result)
}

func manifestPermissions(permissions []apk.UsesPermission) [][]any {
	result := make([][]any, 0, len(permissions))
	for _, permission := range permissions {
		name, err := permission.Name.String()
		if err != nil || name == "" {
			continue
		}
		var maxSDK any
		if value, err := permission.Max.Int32(); err == nil && value > 0 {
			maxSDK = value
		}
		result = append(result, []any{name, maxSDK})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i][0].(string) < result[j][0].(string)
	})
	return result
}

func mapKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
