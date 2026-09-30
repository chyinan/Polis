// pattern: Imperative Shell
//go:build !windows

package qqnotify

func ReadProtectedQQCredentials(string) (Credentials, error) {
	return Credentials{}, ErrProtectedCredentialStoreUnavailable
}

func StoreProtectedQQCredentials(string, Credentials) error {
	return ErrProtectedCredentialStoreUnavailable
}
