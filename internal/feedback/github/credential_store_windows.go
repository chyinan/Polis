// pattern: Imperative Shell
//go:build windows

package github

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const githubCredentialEntropy = "Polis GitHub read-only credential v1"

type protectedGitHubCredentialStore struct {
	root string
}

func NewProtectedGitHubCredentialStore() (GitHubCredentialStore, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" || !filepath.IsAbs(localAppData) {
		return nil, ErrProtectedGitHubCredentialStoreUnavailable
	}
	return &protectedGitHubCredentialStore{root: filepath.Join(localAppData, "Polis", "secure", "github")}, nil
}

func (s *protectedGitHubCredentialStore) StoreToken(credentialRef, token string) error {
	if s == nil || !ValidGitHubCredentialRef(credentialRef) || !ValidGitHubToken(token) {
		return errors.New("GitHub credential input is invalid")
	}
	plain := []byte(token)
	defer zeroGitHubCredentialBytes(plain)
	protected, err := cryptGitHubCredential(plain, true)
	if err != nil || len(protected) > 4096 {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	defer zeroGitHubCredentialBytes(protected)
	if err = os.MkdirAll(s.root, 0700); err != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	temporary, err := os.CreateTemp(s.root, ".github-token-*.tmp")
	if err != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0600); err == nil {
		_, err = temporary.Write(protected)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil || closeErr != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	from, fromErr := windows.UTF16PtrFromString(temporaryPath)
	to, toErr := windows.UTF16PtrFromString(s.credentialPath(credentialRef))
	if fromErr != nil || toErr != nil || windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH) != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	return nil
}

func (s *protectedGitHubCredentialStore) LoadToken(credentialRef string) (string, error) {
	if s == nil || !ValidGitHubCredentialRef(credentialRef) {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	path := s.credentialPath(credentialRef)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > 4096 {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	protected, readErr := io.ReadAll(io.LimitReader(file, 4097))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(protected) == 0 || len(protected) > 4096 {
		zeroGitHubCredentialBytes(protected)
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	plain, err := cryptGitHubCredential(protected, false)
	zeroGitHubCredentialBytes(protected)
	if err != nil {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	defer zeroGitHubCredentialBytes(plain)
	if !ValidGitHubToken(string(plain)) {
		return "", ErrProtectedGitHubCredentialStoreUnavailable
	}
	return string(plain), nil
}

func (s *protectedGitHubCredentialStore) DeleteToken(credentialRef string) error {
	if s == nil || !ValidGitHubCredentialRef(credentialRef) {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	err := os.Remove(s.credentialPath(credentialRef))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return ErrProtectedGitHubCredentialStoreUnavailable
	}
	return nil
}

func (s *protectedGitHubCredentialStore) credentialPath(credentialRef string) string {
	return filepath.Join(s.root, "github-read-"+credentialRef+".dpapi")
}

func cryptGitHubCredential(content []byte, protect bool) ([]byte, error) {
	if len(content) == 0 {
		return nil, errors.New("empty GitHub credential")
	}
	input := windows.DataBlob{Size: uint32(len(content)), Data: &content[0]}
	entropy := []byte(githubCredentialEntropy)
	optionalEntropy := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var output windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&input, nil, &optionalEntropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	} else {
		err = windows.CryptUnprotectData(&input, nil, &optionalEntropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	}
	if err != nil || output.Data == nil || output.Size == 0 {
		return nil, errors.New("Windows data protection failed")
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	result := append([]byte(nil), unsafe.Slice(output.Data, output.Size)...)
	runtime.KeepAlive(content)
	runtime.KeepAlive(entropy)
	return result, nil
}

func zeroGitHubCredentialBytes(content []byte) {
	for index := range content {
		content[index] = 0
	}
}
