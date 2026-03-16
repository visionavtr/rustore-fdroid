package internal

import (
	"fmt"

	"github.com/shogo82148/androidbinary/apk"
)

// ExtractPermissions parses an APK and returns uses-permission entries
// in the F-Droid index format: [name, maxSdkVersion] where maxSdkVersion is null if unset.
func ExtractPermissions(apkPath string) ([][]any, error) {
	a, err := apk.OpenFile(apkPath)
	if err != nil {
		return nil, fmt.Errorf("open APK: %w", err)
	}
	defer a.Close()

	manifest := a.Manifest()
	if len(manifest.UsesPermissions) == 0 {
		return nil, nil
	}

	perms := make([][]any, 0, len(manifest.UsesPermissions))
	for _, p := range manifest.UsesPermissions {
		name, err := p.Name.String()
		if err != nil || name == "" {
			continue
		}
		var maxSDK any
		if v, err := p.Max.Int32(); err == nil && v > 0 {
			maxSDK = v
		}
		perms = append(perms, []any{name, maxSDK})
	}
	return perms, nil
}
