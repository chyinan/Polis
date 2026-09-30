//go:build linux

// pattern: Functional Core

package runner

import (
	"path/filepath"
	"strings"
)

func validLinuxWorkerCgroupSandboxArgv(argv []string, hostCgroupRoot, expectedHome string) bool {
	if len(argv) < 20 || !filepath.IsAbs(argv[0]) || filepath.Clean(argv[0]) != argv[0] || filepath.Base(argv[0]) != "bwrap" ||
		!validLinuxWorkerCgroupRootPath(hostCgroupRoot) || !filepath.IsAbs(expectedHome) || filepath.Clean(expectedHome) != expectedHome ||
		linuxWorkerHostCgroupPath(expectedHome, hostCgroupRoot) {
		return false
	}
	separator := -1
	for index, argument := range argv {
		if argument == "--" {
			if separator >= 0 {
				return false
			}
			separator = index
		}
	}
	if separator < 0 || separator+4 != len(argv) || argv[separator+1] != "/codex" || argv[separator+2] != "app-server" || argv[separator+3] != "--stdio" {
		return false
	}
	index := 1
	consume := func(expected ...string) bool {
		if len(expected) > separator-index {
			return false
		}
		for _, token := range expected {
			if argv[index] != token {
				return false
			}
			index++
		}
		return true
	}
	takePath := func() (string, bool) {
		if index >= separator {
			return "", false
		}
		path := argv[index]
		index++
		return path, filepath.IsAbs(path) && filepath.Clean(path) == path && !linuxWorkerHostCgroupPath(path, hostCgroupRoot)
	}
	if !consume("--unshare-all", "--unshare-cgroup", "--share-net", "--die-with-parent", "--new-session",
		"--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/etc/ssl", "/etc/ssl", "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf",
		"--ro-bind", "/etc/hosts", "/etc/hosts", "--proc", "/proc", "--dev", "/dev",
		"--tmpfs", "/tmp", "--dir", "/sys", "--tmpfs", "/sys", "--dir", "/work", "--bind") {
		return false
	}
	workerHome, ok := takePath()
	if !ok || workerHome != expectedHome || !consume("/home/codex", "--ro-bind") {
		return false
	}
	if _, ok = takePath(); !ok || !consume("/codex", "--chdir", "/work", "--clearenv", "--setenv", "PATH", "/usr/bin:/bin",
		"--setenv", "HOME", "/home/codex", "--setenv", "CODEX_HOME", "/home/codex", "--setenv", "LANG", "C.UTF-8") {
		return false
	}
	if index+2 < separator && argv[index] == "--ro-bind" && argv[index+2] == "/home/codex/auth.json" {
		if !consume("--ro-bind") {
			return false
		}
		if _, ok = takePath(); !ok || !consume("/home/codex/auth.json") {
			return false
		}
	}
	if !consume("--ro-bind") {
		return false
	}
	if _, ok = takePath(); !ok || !consume("/codex-code-mode-host") {
		return false
	}
	if index < separator {
		if !consume("--setenv", "HTTPS_PROXY") || index >= separator {
			return false
		}
		proxy := argv[index]
		index++
		if proxy == "" || !consume("--setenv", "HTTP_PROXY") || index >= separator || argv[index] != proxy {
			return false
		}
		index++
	}
	return index == separator
}

func validLinuxWorkerCgroupRootPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != string(filepath.Separator)
}

func linuxWorkerHostCgroupPath(path, configuredRoot string) bool {
	clean := filepath.Clean(path)
	for _, root := range []string{"/sys/fs/cgroup", "/sys/cgroup", filepath.Clean(configuredRoot)} {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
