// pattern: Functional Core
package probe

import "testing"

func TestDeriveFrontendTerminalStatus(t *testing.T) {
	tests := []struct {
		name  string
		input FrontendTerminalExecution
		want  string
	}{
		{
			name:  "pre-provider failure remains not started",
			input: FrontendTerminalExecution{},
			want:  FrontendTerminalNotStarted,
		},
		{
			name: "provider transport is inconclusive",
			input: FrontendTerminalExecution{
				AllowanceConsumed: true,
				SessionCreated:    true,
				ProviderEgress:    true,
				TransportOutcome:  "PROVIDER_STREAM_NONRECOVERABLE",
				BusinessVerdict:   FrontendTerminalFailed,
			},
			want: FrontendTerminalInconclusive,
		},
		{
			name: "completed business failure does not need an artifact",
			input: FrontendTerminalExecution{
				AllowanceConsumed: true,
				SessionCreated:    true,
				ProviderEgress:    true,
				BusinessVerdict:   FrontendTerminalFailed,
			},
			want: FrontendTerminalFailed,
		},
		{
			name: "successful business is passed",
			input: FrontendTerminalExecution{
				AllowanceConsumed: true,
				SessionCreated:    true,
				ProviderEgress:    true,
				BusinessVerdict:   FrontendTerminalPassed,
			},
			want: FrontendTerminalPassed,
		},
		{
			name: "allowance consumed before session failure is not not started",
			input: FrontendTerminalExecution{
				AllowanceConsumed: true,
			},
			want: FrontendTerminalInconclusive,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DeriveFrontendTerminalStatus(test.input); got != test.want {
				t.Fatalf("DeriveFrontendTerminalStatus() = %q, want %q", got, test.want)
			}
		})
	}
}
