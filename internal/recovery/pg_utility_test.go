// pattern: Imperative Shell
package recovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresUtilityTargetKeepsPasswordOutOfServiceAndProcessArguments(t *testing.T) {
	directory := t.TempDir()
	const secret = "restore-secret-value"
	serviceName, environment, cleanup, err := preparePostgresUtilityTarget("postgres://restore_user:"+secret+"@127.0.0.1:5432/polis_restore?sslmode=require", "", directory)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if strings.Contains(serviceName, secret) {
		t.Fatal("PostgreSQL service name contains the DSN password")
	}
	var serviceFile, passFile string
	for _, item := range environment {
		if strings.HasPrefix(item, "PGSERVICEFILE=") {
			serviceFile = strings.TrimPrefix(item, "PGSERVICEFILE=")
		}
		if strings.HasPrefix(item, "PGPASSFILE=") {
			passFile = strings.TrimPrefix(item, "PGPASSFILE=")
		}
	}
	serviceBytes, err := os.ReadFile(serviceFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serviceBytes), secret) || !strings.Contains(string(serviceBytes), "sslmode=require") {
		t.Fatal("service configuration exposed the password or lost the TLS mode")
	}
	passBytes, err := os.ReadFile(passFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(passBytes), secret) {
		t.Fatal("temporary pgpass file omitted the password")
	}
	if filepath.Dir(serviceFile) != directory || filepath.Dir(passFile) != directory {
		t.Fatal("temporary PostgreSQL configuration escaped its private directory")
	}
}
