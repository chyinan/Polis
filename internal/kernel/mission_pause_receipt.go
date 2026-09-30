// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

func (k *Kernel) LookupMissionPauseCommand(ctx context.Context, scope Scope, missionID string, paused bool, requestID string) (Receipt, bool, error) {
	if !core.ValidID(missionID) || !core.ValidID(requestID) {
		return Receipt{}, false, core.Malformed
	}
	operation := "mission.resume"
	if paused {
		operation = "mission.pause"
	}
	inputHash := fingerprint(struct {
		Op    string
		Input any
	}{operation, struct {
		ID     string
		Paused bool
	}{missionID, paused}})
	var storedHash string
	var encoded []byte
	err := k.pool.QueryRow(ctx, `SELECT fingerprint,result FROM receipts WHERE company_id=$1 AND actor='local-owner' AND key=$2`, scope.company, requestID).Scan(&storedHash, &encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, err
	}
	if storedHash != inputHash {
		return Receipt{}, false, core.Conflict
	}
	var receipt Receipt
	if err = json.Unmarshal(encoded, &receipt); err != nil {
		return Receipt{}, false, core.Integrity
	}
	return receipt, true, nil
}
