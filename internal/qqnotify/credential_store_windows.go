// pattern: Imperative Shell
//go:build windows

package qqnotify

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const credentialEntropy = "Polis QQ Bot credential store v1"

func ReadProtectedQQCredentials(credentialRef string) (Credentials, error) {
	path, err := protectedCredentialPath(credentialRef)
	if err != nil {
		return Credentials{}, err
	}
	protected, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, ErrProtectedCredentialStoreUnavailable
		}
		return Credentials{}, errors.New("could not read the protected QQ credential store")
	}
	plain, err := cryptCredentialBlob(protected, false)
	if err != nil {
		return Credentials{}, errors.New("could not decrypt the protected QQ credential store")
	}
	defer zeroCredentialBytes(plain)
	decoder := json.NewDecoder(bytes.NewReader(plain))
	decoder.DisallowUnknownFields()
	var credentials Credentials
	if err = decoder.Decode(&credentials); err != nil {
		return Credentials{}, errors.New("protected QQ credential data is invalid")
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Credentials{}, errors.New("protected QQ credential data has trailing content")
	}
	if err = credentials.Validate(); err != nil {
		return Credentials{}, errors.New("protected QQ credential data is invalid")
	}
	return credentials, nil
}

func StoreProtectedQQCredentials(credentialRef string, credentials Credentials) error {
	if err := credentials.Validate(); err != nil {
		return err
	}
	plain, err := json.Marshal(credentials)
	if err != nil {
		return errors.New("could not encode QQ credential data")
	}
	defer zeroCredentialBytes(plain)
	protected, err := cryptCredentialBlob(plain, true)
	if err != nil {
		return errors.New("could not protect QQ credential data with the Windows user profile")
	}
	path, err := protectedCredentialPath(credentialRef)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err = os.MkdirAll(directory, 0700); err != nil {
		return errors.New("could not create the protected QQ credential directory")
	}
	temporary, err := os.CreateTemp(directory, ".qq-credentials-*.tmp")
	if err != nil {
		return errors.New("could not create a temporary protected credential file")
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
		return errors.New("could not persist protected QQ credentials")
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return errors.New("could not atomically replace protected QQ credentials")
	}
	return nil
}

func protectedCredentialPath(credentialRef string) (string, error) {
	if !ValidCredentialRef(credentialRef) {
		return "", errors.New("credential reference is invalid")
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" || !filepath.IsAbs(localAppData) {
		return "", errors.New("LOCALAPPDATA is unavailable for protected QQ credentials")
	}
	return filepath.Join(localAppData, "Polis", "secure", "qqbot-"+credentialRef+".dpapi"), nil
}

func cryptCredentialBlob(content []byte, protect bool) ([]byte, error) {
	if len(content) == 0 {
		return nil, errors.New("empty credential data")
	}
	input := windows.DataBlob{Size: uint32(len(content)), Data: &content[0]}
	entropy := []byte(credentialEntropy)
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
	protected := unsafe.Slice(output.Data, output.Size)
	result := append([]byte(nil), protected...)
	runtime.KeepAlive(content)
	runtime.KeepAlive(entropy)
	return result, nil
}

func zeroCredentialBytes(content []byte) {
	for index := range content {
		content[index] = 0
	}
}
