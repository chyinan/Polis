package control

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestNormalizeOpenAIEndpoint(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{raw: "https://api.openai.com/v1/", want: "https://api.openai.com/v1", ok: true},
		{raw: "https://example.invalid/v1", want: "https://example.invalid/v1", ok: true},
		{raw: "http://127.0.0.1:8080/v1", want: "http://127.0.0.1:8080/v1", ok: true},
		{raw: "http://example.com/v1", ok: false},
		{raw: "https://user:key@api.openai.com/v1", ok: false},
		{raw: "https://api.openai.com/v1?token=secret", ok: false},
		{raw: "ftp://api.openai.com/v1", ok: false},
	}
	for _, tc := range cases {
		got, err := normalizeOpenAIEndpoint(tc.raw)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("normalize(%q) = %q, %v; want %q", tc.raw, got, err, tc.want)
		}
		if !tc.ok && !errors.Is(err, core.Malformed) {
			t.Fatalf("normalize(%q) err = %v, want malformed", tc.raw, err)
		}
	}
}

func TestNormalizeDeepSeekEffort(t *testing.T) {
	for _, effort := range []string{"none", "low", "high", "max"} {
		got, err := normalizeDeepSeekEffort(effort)
		if err != nil || got != effort {
			t.Fatalf("normalize(%q) = %q, %v", effort, got, err)
		}
	}
	if _, err := normalizeDeepSeekEffort("chat"); !errors.Is(err, core.Malformed) {
		t.Fatalf("chat effort err = %v, want malformed", err)
	}
}

func TestUpdateRuntimeSettingsRejectsNonLocalOpenAIHTTP(t *testing.T) {
	service := NewService(nil, nil)
	_, err := service.UpdateRuntimeSettings(context.Background(), "company-1", UpdateRuntimeSettingsRequest{
		Provider: "openai", Model: "gpt-4o-mini", Effort: "chat", Profile: "http://example.com/v1", RequestID: "runtime-openai-1",
	})
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("err = %v, want malformed", err)
	}
}
