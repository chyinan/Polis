// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

// ProviderLifecycleEvent is bounded controller evidence for the provider
// reservation-to-initialize bridge. It deliberately excludes prompts,
// credentials, environment values and unrestricted process output.
type ProviderLifecycleEvent struct {
	Phase          string `json:"phase"`
	ReasonCode     string `json:"reason_code,omitempty"`
	SafeMessage    string `json:"safe_message,omitempty"`
	ProcessPID     int    `json:"process_pid,omitempty"`
	ProcessCreated bool   `json:"process_created,omitempty"`
	PIDAttached    bool   `json:"pid_attached,omitempty"`
}

func (k *Kernel) TXRecordProviderLifecycle(ctx context.Context, b Binding, event ProviderLifecycleEvent, key string) (Receipt, error) {
	if event.Phase == "" || key == "" {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, b.scope, nil, key, "provider.runtime.lifecycle", event, func(tx pgx.Tx) (Receipt, error) {
		if _, err := k.checkSession(ctx, tx, b, false); err != nil {
			return Receipt{}, err
		}
		data, err := json.Marshal(event)
		if err != nil {
			return Receipt{}, err
		}
		id := newID()
		if _, err = tx.Exec(ctx, "INSERT INTO worker_observations(company_id,id,session_id,reason,data) VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, "provider_lifecycle", data); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: event.Phase}, nil
	})
}
