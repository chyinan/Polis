// pattern: Imperative Shell
package recovery

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var postgresKeywordParameterPattern = regexp.MustCompile(`(?i)(?:^|\s)%s\s*=\s*(?:'((?:\\.|[^'])*)'|"((?:\\.|[^"])*)"|([^\s]+))`)

func preparePostgresUtilityTarget(dsn, databaseOverride, temporaryDirectory string) (string, []string, func(), error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("%w: PostgreSQL DSN could not be parsed", ErrRecoveryBackupInvalid)
	}
	if config.Database == "" || config.User == "" || config.Port == 0 || config.Host == "" {
		return "", nil, func() {}, fmt.Errorf("%w: PostgreSQL utility connection requires explicit host, port, database and user", ErrRecoveryBackupInvalid)
	}
	if strings.Contains(config.Host, ",") || strings.Contains(config.Host, "\x00") {
		return "", nil, func() {}, fmt.Errorf("%w: multi-host or invalid PostgreSQL utility connections are unsupported", ErrRecoveryBackupInvalid)
	}
	if !filepath.IsAbs(temporaryDirectory) {
		return "", nil, func() {}, fmt.Errorf("%w: PostgreSQL utility configuration directory must be absolute", ErrRecoveryBackupInvalid)
	}
	serviceName, err := newRecoveryGenerationID()
	if err != nil {
		return "", nil, func() {}, err
	}
	serviceName = "polis_recovery_" + serviceName
	databaseName := config.Database
	if databaseOverride != "" {
		databaseName = databaseOverride
	}
	parameters, err := postgresDSNParameters(dsn)
	if err != nil {
		return "", nil, func() {}, err
	}
	if _, found := parameters["service"]; found {
		return "", nil, func() {}, fmt.Errorf("%w: service indirection is not allowed for recovery commands", ErrRecoveryBackupInvalid)
	}
	if _, found := parameters["servicefile"]; found {
		return "", nil, func() {}, fmt.Errorf("%w: service-file indirection is not allowed for recovery commands", ErrRecoveryBackupInvalid)
	}
	if _, found := parameters["passfile"]; found {
		return "", nil, func() {}, fmt.Errorf("%w: caller passfile paths are not inherited by recovery commands", ErrRecoveryBackupInvalid)
	}
	if _, found := parameters["sslpassword"]; found {
		return "", nil, func() {}, fmt.Errorf("%w: encrypted PostgreSQL client-key passwords are not supported by the recovery utility wrapper", ErrRecoveryBackupInvalid)
	}
	serviceParameters := map[string]string{
		"host":   config.Host,
		"port":   fmt.Sprint(config.Port),
		"dbname": databaseName,
		"user":   config.User,
	}
	if hostaddr := parameters["hostaddr"]; hostaddr != "" {
		serviceParameters["hostaddr"] = hostaddr
	}
	sslMode := parameters["sslmode"]
	if sslMode == "" {
		if config.TLSConfig != nil {
			sslMode = "prefer"
		} else {
			sslMode = "disable"
		}
	}
	serviceParameters["sslmode"] = sslMode
	for _, key := range []string{"sslrootcert", "sslcert", "sslkey", "sslcrl", "sslsni", "sslnegotiation", "gssencmode", "channel_binding", "target_session_attrs", "load_balance_hosts", "connect_timeout", "application_name", "options"} {
		if value := parameters[key]; value != "" {
			serviceParameters[key] = value
		}
	}
	var serviceBuilder strings.Builder
	serviceBuilder.WriteString("[" + serviceName + "]\n")
	for _, key := range []string{"host", "hostaddr", "port", "dbname", "user", "sslmode", "sslrootcert", "sslcert", "sslkey", "sslcrl", "sslsni", "sslnegotiation", "gssencmode", "channel_binding", "target_session_attrs", "load_balance_hosts", "connect_timeout", "application_name", "options"} {
		if value, found := serviceParameters[key]; found {
			if strings.ContainsAny(value, "\r\n\x00") || strings.TrimSpace(value) != value {
				return "", nil, func() {}, fmt.Errorf("%w: PostgreSQL utility connection parameter contains line-breaking or ambiguous whitespace", ErrRecoveryBackupInvalid)
			}
			serviceBuilder.WriteString(key + "=" + value + "\n")
		}
	}
	servicePath := filepath.Join(temporaryDirectory, "pg_service.conf")
	if err = writeSyncedFile(servicePath, []byte(serviceBuilder.String()), 0o600); err != nil {
		return "", nil, func() {}, err
	}
	passPath := filepath.Join(temporaryDirectory, "pgpass")
	passRecord := ""
	if config.Password != "" {
		passHost := config.Host
		if filepath.IsAbs(passHost) {
			passHost = "*"
		}
		passRecord = strings.Join([]string{escapePassfileField(passHost), fmt.Sprint(config.Port), escapePassfileField(databaseName), escapePassfileField(config.User), escapePassfileField(config.Password)}, ":") + "\n"
	}
	if err = writeSyncedFile(passPath, []byte(passRecord), 0o600); err != nil {
		_ = os.Remove(servicePath)
		return "", nil, func() {}, err
	}
	environment := []string{"PGSERVICEFILE=" + servicePath, "PGSERVICE=" + serviceName, "PGPASSFILE=" + passPath}
	cleanup := func() {
		_ = os.Remove(servicePath)
		_ = os.Remove(passPath)
	}
	return serviceName, environment, cleanup, nil
}

func postgresDSNParameters(dsn string) (map[string]string, error) {
	parameters := make(map[string]string)
	if strings.HasPrefix(strings.ToLower(dsn), "postgres://") || strings.HasPrefix(strings.ToLower(dsn), "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return nil, fmt.Errorf("%w: PostgreSQL URI could not be parsed", ErrRecoveryBackupInvalid)
		}
		for key, values := range parsed.Query() {
			if len(values) != 1 {
				return nil, fmt.Errorf("%w: repeated PostgreSQL URI parameter is unsupported", ErrRecoveryBackupInvalid)
			}
			parameters[strings.ToLower(key)] = values[0]
		}
		return parameters, nil
	}
	for _, key := range []string{"service", "servicefile", "passfile", "sslpassword", "hostaddr", "sslmode", "sslrootcert", "sslcert", "sslkey", "sslcrl", "sslsni", "sslnegotiation", "gssencmode", "channel_binding", "target_session_attrs", "load_balance_hosts", "connect_timeout", "application_name", "options"} {
		pattern := regexp.MustCompile(fmt.Sprintf(postgresKeywordParameterPattern.String(), regexp.QuoteMeta(key)))
		match := pattern.FindStringSubmatch(dsn)
		if len(match) == 0 {
			continue
		}
		value := match[1]
		if value == "" {
			value = match[2]
		}
		if value == "" {
			value = match[3]
		}
		value = strings.ReplaceAll(value, "\\'", "'")
		value = strings.ReplaceAll(value, "\\\\", "\\")
		parameters[key] = value
	}
	return parameters, nil
}

func escapePassfileField(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, ":", "\\:")
}

func postgresUtilityEnvironment(overrides []string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		name, _, found := strings.Cut(item, "=")
		if found && strings.HasPrefix(strings.ToUpper(name), "PG") {
			continue
		}
		environment = append(environment, item)
	}
	return append(environment, overrides...)
}
