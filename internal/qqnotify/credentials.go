// pattern: Functional Core
package qqnotify

import (
	"errors"
	"strings"
	"unicode"
)

var ErrProtectedCredentialStoreUnavailable = errors.New("protected QQ credential storage is unavailable")

type Credentials struct {
	AppID        string `json:"appId"`
	ClientSecret string `json:"clientSecret"`
}

func (credentials Credentials) Validate() error {
	if strings.TrimSpace(credentials.AppID) == "" || strings.TrimSpace(credentials.ClientSecret) == "" || len(credentials.AppID) > 128 || len(credentials.ClientSecret) > 512 {
		return errors.New("QQ Bot credentials are empty or exceed the accepted size")
	}
	for _, value := range []string{credentials.AppID, credentials.ClientSecret} {
		if strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return errors.New("QQ Bot credentials contain a control character")
		}
	}
	return nil
}

func ValidCredentialRef(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || strings.ContainsRune("_-", character) {
			continue
		}
		return false
	}
	return true
}
