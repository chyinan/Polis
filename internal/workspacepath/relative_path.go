// pattern: Functional Core
package workspacepath

import (
	"strings"
	"unicode/utf8"
)

func ValidRelativePath(path string) bool {
	if path == "" || !utf8.ValidString(path) || len(path) > 1024 || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.ContainsAny(path, "\\%:\x00") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." || len(segment) > 255 || strings.TrimSpace(segment) != segment {
			return false
		}
		for _, character := range segment {
			if character < 0x20 || character == 0x7f {
				return false
			}
		}
	}
	return true
}
