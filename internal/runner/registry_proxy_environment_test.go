// pattern: Functional Core
package runner

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildRegistryProxyEnvironmentPinsBothProxyVariables(t *testing.T) {
	base := []string{
		"HOME=C:\\work\\home", "LOCALAPPDATA=C:\\work", "PATH=C:\\Windows\\System32",
		"SYSTEMROOT=C:\\Windows", "TEMP=C:\\work\\tmp", "TMP=C:\\work\\tmp", "USERPROFILE=C:\\work\\home",
	}
	proxyURL := "http://polis:lease-secret@127.0.0.1:43123"
	result, err := BuildRegistryProxyEnvironment(base, proxyURL)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string)
	for _, entry := range result {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("invalid environment entry %q", entry)
		}
		values[name] = value
	}
	if values["HTTP_PROXY"] != proxyURL || values["HTTPS_PROXY"] != proxyURL {
		t.Fatalf("proxy environment = %v", values)
	}
	if !reflect.DeepEqual(base, []string{
		"HOME=C:\\work\\home", "LOCALAPPDATA=C:\\work", "PATH=C:\\Windows\\System32",
		"SYSTEMROOT=C:\\Windows", "TEMP=C:\\work\\tmp", "TMP=C:\\work\\tmp", "USERPROFILE=C:\\work\\home",
	}) {
		t.Fatal("building proxy environment mutated the input")
	}
}

func TestBuildRegistryProxyEnvironmentRejectsProxyOverridesAndBypass(t *testing.T) {
	base := []string{
		"HOME=/work/home", "LOCALAPPDATA=/work", "PATH=/bin", "SYSTEMROOT=/system",
		"TEMP=/work/tmp", "TMP=/work/tmp", "USERPROFILE=/work/home",
	}
	for name, entries := range map[string][]string{
		"different HTTP proxy":  append(append([]string(nil), base...), "HTTP_PROXY=http://attacker.invalid"),
		"different HTTPS proxy": append(append([]string(nil), base...), "HTTPS_PROXY=http://attacker.invalid"),
		"proxy bypass":          append(append([]string(nil), base...), "NO_PROXY=registry.npmjs.org"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildRegistryProxyEnvironment(entries, "http://polis:lease-secret@127.0.0.1:43123"); err == nil {
				t.Fatal("registry proxy override or bypass was accepted")
			}
		})
	}
}

func TestBuildRegistryProxyEnvironmentRequiresCredentialedCanonicalLoopbackURL(t *testing.T) {
	base := []string{
		"HOME=/work/home", "LOCALAPPDATA=/work", "PATH=/bin", "SYSTEMROOT=/system",
		"TEMP=/work/tmp", "TMP=/work/tmp", "USERPROFILE=/work/home",
	}
	for _, proxyURL := range []string{
		"http://127.0.0.1:43123",
		"http://user@127.0.0.1:43123",
		"http://user:secret@127.0.0.2:43123",
		"http://user:secret@[::1]:43123",
		"http://user:secret@registry.example:43123",
		"http://user:secret@127.0.0.1:43123/path",
		"http://user:secret@127.0.0.1:43123?bypass=1",
	} {
		t.Run(proxyURL, func(t *testing.T) {
			if _, err := BuildRegistryProxyEnvironment(base, proxyURL); err == nil {
				t.Fatalf("accepted non-credentialed or non-canonical proxy URL %q", proxyURL)
			}
		})
	}
}
