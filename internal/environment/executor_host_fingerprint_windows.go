//go:build windows

// pattern: Imperative Shell
package environment

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows/registry"
)

func currentHostBuildIdentity() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer key.Close()
	build, _, err := key.GetStringValue("CurrentBuildNumber")
	if err != nil || build == "" {
		return "", fmt.Errorf("Windows build number is unavailable")
	}
	version, _, _ := key.GetStringValue("DisplayVersion")
	ubr, _, _ := key.GetIntegerValue("UBR")
	product, _, _ := key.GetStringValue("ProductName")
	return fmt.Sprintf("windows\x00%s\x00%s\x00%s\x00%d", runtime.GOARCH, product, version, ubr), nil
}
