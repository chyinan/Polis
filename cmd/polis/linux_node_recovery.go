// pattern: Functional Core
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func validateLinuxNodeStartupRecoveryRoot(hostOS string, hasUnrestoredWork bool, runtimeRoot, cgroupRoot string) error {
	if hostOS != "linux" {
		return nil
	}
	if !hasUnrestoredWork && strings.TrimSpace(cgroupRoot) == "" {
		return nil
	}
	if strings.TrimSpace(runtimeRoot) == "" || strings.TrimSpace(cgroupRoot) == "" {
		return errors.New("Linux Node runtime root and delegated cgroup root are required to reconcile host processes safely")
	}
	return nil
}

func linuxNodeInstanceIdentity(databaseDSN, blobRoot string) (string, error) {
	if strings.TrimSpace(databaseDSN) == "" || strings.TrimSpace(blobRoot) == "" || !filepath.IsAbs(blobRoot) {
		return "", errors.New("Linux Node instance identity requires a database DSN and canonical blob root")
	}
	config, err := pgx.ParseConfig(databaseDSN)
	if err != nil {
		return "", errors.New("Linux Node instance identity cannot parse the configured database connection")
	}
	if strings.TrimSpace(config.Database) == "" || strings.TrimSpace(config.User) == "" {
		return "", errors.New("Linux Node instance identity requires a database name and role")
	}
	if strings.TrimSpace(config.Host) == "" || config.Port <= 0 {
		return "", errors.New("Linux Node instance identity requires a PostgreSQL host and port")
	}
	host := strings.TrimSpace(config.Host)
	if strings.Contains(host, string(filepath.Separator)) {
		host = filepath.Clean(host)
	} else {
		host = strings.ToLower(host)
	}
	identityInput := strings.Join([]string{
		"polis-linux-node-instance@2", host, strconv.Itoa(int(config.Port)), config.Database, config.User, filepath.Clean(blobRoot),
	}, "\x00")
	digest := sha256.Sum256([]byte(identityInput))
	return hex.EncodeToString(digest[:]), nil
}
