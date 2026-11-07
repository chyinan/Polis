// pattern: Functional Core
package desktop

import "testing"

func TestTokenMatchesRejectsBlankOrWrongToken(t *testing.T) {
	for _, test := range []struct {
		name      string
		expected  string
		presented string
		want      bool
	}{
		{name: "blank expected", expected: "", presented: "token", want: false},
		{name: "blank presented", expected: "token", presented: "", want: false},
		{name: "wrong", expected: "token", presented: "other", want: false},
		{name: "exact", expected: "token", presented: "token", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := TokenMatches(test.expected, test.presented); got != test.want {
				t.Fatalf("TokenMatches(%q,%q)=%v, want %v", test.expected, test.presented, got, test.want)
			}
		})
	}
}

func TestAllowedOriginIsNarrow(t *testing.T) {
	if !AllowedOrigin("tauri://localhost") || !AllowedOrigin("") {
		t.Fatal("expected desktop and empty probe origins to be allowed")
	}
	if AllowedOrigin("https://attacker.example") {
		t.Fatal("unexpected third-party origin allowed")
	}
}

func TestPresentedTokenPrefersHeaderThenBearerThenEventSourceQuery(t *testing.T) {
	if got := PresentedToken("header", "Bearer bearer", "query"); got != "header" {
		t.Fatalf("header token=%q", got)
	}
	if got := PresentedToken("", "Bearer bearer", "query"); got != "bearer" {
		t.Fatalf("bearer token=%q", got)
	}
	if got := PresentedToken("", "", "query"); got != "query" {
		t.Fatalf("query token=%q", got)
	}
}

func TestValidateServerBindingRequiresTokenForNonLoopback(t *testing.T) {
	for _, test := range []struct {
		name    string
		address string
		token   string
		wantErr bool
	}{
		{name: "loopback tokenless", address: "127.0.0.1:8080", token: "", wantErr: false},
		{name: "wildcard tokenless", address: ":8080", token: "", wantErr: true},
		{name: "public tokenless", address: "0.0.0.0:8080", token: "", wantErr: true},
		{name: "public tokenized", address: "0.0.0.0:8080", token: "session", wantErr: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateServerBinding(test.address, test.token); (err != nil) != test.wantErr {
				t.Fatalf("ValidateServerBinding(%q,%q) error=%v, wantErr=%v", test.address, test.token, err, test.wantErr)
			}
		})
	}
}
