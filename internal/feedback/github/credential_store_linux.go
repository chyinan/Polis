// pattern: Imperative Shell
//go:build linux

package github

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	linuxGitHubCredentialService = "polis-github-readonly"
	linuxGitHubCredentialTimeout = 5 * time.Second
	linuxGitHubCredentialOutput  = 513
)

type linuxSecretToolRunner interface {
	run(context.Context, string, []string, []byte, int) ([]byte, error)
}

type linuxProtectedGitHubCredentialStore struct {
	executable string
	runner     linuxSecretToolRunner
}

type commandSecretToolRunner struct{}

func NewProtectedGitHubCredentialStore() (GitHubCredentialStore, error) {
	if strings.TrimSpace(os.Getenv("DBUS_SESSION_BUS_ADDRESS")) == "" {
		return nil, ErrProtectedGitHubCredentialStoreUnavailable
	}
	executable, err := exec.LookPath("secret-tool")
	if err != nil {
		return nil, ErrProtectedGitHubCredentialStoreUnavailable
	}
	return &linuxProtectedGitHubCredentialStore{executable: executable, runner: commandSecretToolRunner{}}, nil
}

func (store *linuxProtectedGitHubCredentialStore) StoreToken(credentialRef, token string) error {
	if store == nil || store.runner == nil || store.executable == "" || !ValidGitHubCredentialRef(credentialRef) || !ValidGitHubToken(token) {
		return errors.New("GitHub credential input is invalid")
	}
	secret := []byte(token)
	defer zeroLinuxCredentialBytes(secret)
	_, err := store.runner.run(context.Background(), store.executable, []string{
		"store", "--label", "Polis GitHub read-only credential",
		"service", linuxGitHubCredentialService, "account", credentialRef,
	}, secret, 0)
	if err != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	return nil
}

func (store *linuxProtectedGitHubCredentialStore) LoadToken(credentialRef string) (string, error) {
	if store == nil || store.runner == nil || store.executable == "" || !ValidGitHubCredentialRef(credentialRef) {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	secret, err := store.runner.run(context.Background(), store.executable, []string{
		"lookup", "service", linuxGitHubCredentialService, "account", credentialRef,
	}, nil, linuxGitHubCredentialOutput)
	if err != nil {
		zeroLinuxCredentialBytes(secret)
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	if bytes.HasSuffix(secret, []byte("\r\n")) {
		secret = secret[:len(secret)-2]
	} else if bytes.HasSuffix(secret, []byte("\n")) {
		secret = secret[:len(secret)-1]
	}
	token := string(secret)
	zeroLinuxCredentialBytes(secret)
	if !ValidGitHubToken(token) {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	return token, nil
}

func (store *linuxProtectedGitHubCredentialStore) DeleteToken(credentialRef string) error {
	if store == nil || store.runner == nil || store.executable == "" || !ValidGitHubCredentialRef(credentialRef) {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	_, err := store.runner.run(context.Background(), store.executable, []string{
		"clear", "service", linuxGitHubCredentialService, "account", credentialRef,
	}, nil, 0)
	if err != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	return nil
}

func (commandSecretToolRunner) run(parent context.Context, executable string, args []string, input []byte, maxOutput int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, linuxGitHubCredentialTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = secretServiceEnvironment(os.Environ())
	command.Stdin = bytes.NewReader(input)
	command.Stderr = io.Discard
	output := &boundedSecretOutput{limit: maxOutput}
	if maxOutput > 0 {
		command.Stdout = output
	}
	if err := command.Run(); err != nil {
		zeroLinuxCredentialBytes(output.data.Bytes())
		return nil, ErrProtectedGitHubCredentialStoreUnavailable
	}
	result := append([]byte(nil), output.data.Bytes()...)
	zeroLinuxCredentialBytes(output.data.Bytes())
	return result, nil
}

func secretServiceEnvironment(environment []string) []string {
	allowed := map[string]struct{}{
		"DBUS_SESSION_BUS_ADDRESS": {}, "DBUS_SESSION_BUS_PID": {}, "GNOME_KEYRING_CONTROL": {},
		"HOME": {}, "PATH": {}, "XDG_RUNTIME_DIR": {},
	}
	result := make([]string, 0, len(allowed))
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, keep := allowed[name]; keep {
			result = append(result, entry)
		}
	}
	return result
}

type boundedSecretOutput struct {
	data  bytes.Buffer
	limit int
}

func (output *boundedSecretOutput) Write(content []byte) (int, error) {
	if output == nil || output.limit < 0 {
		return 0, ErrProtectedGitHubCredentialStoreUnavailable
	}
	if len(content) > output.limit-output.data.Len() {
		return 0, ErrProtectedGitHubCredentialStoreUnavailable
	}
	return output.data.Write(content)
}

func zeroLinuxCredentialBytes(content []byte) {
	for index := range content {
		content[index] = 0
	}
}

var _ GitHubCredentialStore = (*linuxProtectedGitHubCredentialStore)(nil)
