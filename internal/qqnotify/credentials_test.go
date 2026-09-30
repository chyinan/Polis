// pattern: Functional Core
package qqnotify

import "testing"

func TestCredentialsValidateWithoutExposingValues(t *testing.T) {
	if err := (Credentials{AppID: "app-123", ClientSecret: "secret-value"}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []Credentials{{AppID: "", ClientSecret: "secret"}, {AppID: "app", ClientSecret: "secret\nvalue"}} {
		if err := candidate.Validate(); err == nil {
			t.Fatal("accepted malformed QQ credentials")
		}
	}
}
