package installationauth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	password := []byte("correct horse battery staple")
	encoded, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected Argon2id encoding: %q", encoded)
	}
	if !VerifyPassword(encoded, password) {
		t.Fatal("correct password did not verify")
	}
	if VerifyPassword(encoded, []byte("different password")) {
		t.Fatal("incorrect password verified")
	}
}

func TestPasswordHashRejectsMalformedAndUnboundedParameters(t *testing.T) {
	for _, encoded := range []string{
		"",
		"$argon2id$v=19$m=4294967295,t=4294967295,p=255$AA$AA",
		"$argon2id$v=19$m=65536,t=3,p=4$not-base64$AA",
		"$argon2id$v=16$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		if VerifyPassword(encoded, []byte("password")) {
			t.Fatalf("malformed hash verified: %q", encoded)
		}
	}
	if _, err := HashPassword(nil); err == nil {
		t.Fatal("empty password was accepted")
	}
	if _, err := HashPassword(make([]byte, maxPasswordBytes+1)); err == nil {
		t.Fatal("oversize password was accepted")
	}
}
