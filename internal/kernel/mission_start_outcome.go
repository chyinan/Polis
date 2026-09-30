// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type MissionStartOutcome struct {
	Status             string
	RuntimeIncarnation string
}

type missionStartOutcomeInput struct {
	MissionID          string
	Status             string
	RuntimeIncarnation string
}

func (k *Kernel) TXRecordMissionStartOutcome(ctx context.Context, scope Scope, missionID, startRequestID, status string) (Receipt, error) {
	if !core.ValidID(scope.company) || !core.ValidID(missionID) || !core.ValidID(startRequestID) || (status != "started" && status != "outcome_unknown") {
		return Receipt{}, core.Malformed
	}
	input := missionStartOutcomeInput{MissionID: missionID, Status: status, RuntimeIncarnation: k.incarnation}
	return k.TXWrite(ctx, scope, nil, missionStartOutcomeReceiptKey(missionID, startRequestID), "mission.start.outcome", input, func(tx pgx.Tx) (Receipt, error) {
		state, err := missionState(ctx, tx, scope, missionID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "active" {
			return Receipt{}, core.ConflictError{Reason: "Mission start outcome can only be recorded while the Mission is active", CurrentState: state}
		}
		return Receipt{ID: missionID, Status: status, RuntimeIncarnation: k.incarnation}, nil
	})
}

func (k *Kernel) LookupMissionStartOutcome(ctx context.Context, scope Scope, missionID, startRequestID string) (MissionStartOutcome, bool, error) {
	if !core.ValidID(scope.company) || !core.ValidID(missionID) || !core.ValidID(startRequestID) {
		return MissionStartOutcome{}, false, core.Malformed
	}
	var storedHash string
	var encoded []byte
	err := k.pool.QueryRow(ctx, `SELECT fingerprint,result FROM receipts WHERE company_id=$1 AND actor='local-owner' AND key=$2`, scope.company, missionStartOutcomeReceiptKey(missionID, startRequestID)).Scan(&storedHash, &encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionStartOutcome{}, false, nil
	}
	if err != nil {
		return MissionStartOutcome{}, false, err
	}
	var receipt Receipt
	if err = json.Unmarshal(encoded, &receipt); err != nil {
		return MissionStartOutcome{}, false, core.Integrity
	}
	if receipt.ID != missionID || (receipt.Status != "started" && receipt.Status != "outcome_unknown") || !core.ValidID(receipt.RuntimeIncarnation) {
		return MissionStartOutcome{}, false, core.Integrity
	}
	input := missionStartOutcomeInput{MissionID: missionID, Status: receipt.Status, RuntimeIncarnation: receipt.RuntimeIncarnation}
	wantHash := fingerprint(struct {
		Op    string
		Input any
	}{"mission.start.outcome", input})
	if storedHash != wantHash {
		return MissionStartOutcome{}, false, core.Integrity
	}
	return MissionStartOutcome{Status: receipt.Status, RuntimeIncarnation: receipt.RuntimeIncarnation}, true, nil
}

func missionStartOutcomeReceiptKey(missionID, startRequestID string) string {
	digest := sha256.Sum256([]byte(missionID + "\x00" + startRequestID))
	return "mission-start-outcome-" + hex.EncodeToString(digest[:12])
}
