// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/intake"
)

// ReadProductTaskCSVRange serves an exact bounded range only to the active
// WorkerSession bound to the Task whose immutable manifest contains the source.
// The append-only event records the source and range digests without copying
// cell content into the event ledger.
func (k *Kernel) ReadProductTaskCSVRange(ctx context.Context, binding Binding, inputID string, revision int64, sourceDigest, manifestDigest string, startRow, maxRows int) (intake.CSVRowRange, error) {
	if !core.ValidID(binding.session) || !core.ValidID(binding.task) || !core.ValidID(inputID) || revision < 1 ||
		!validTaskInputDigest(sourceDigest) || !validTaskInputDigest(manifestDigest) || startRow < 0 || maxRows < 1 || maxRows > intake.MaxCSVRangeRows {
		return intake.CSVRowRange{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return intake.CSVRowRange{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.guardWithSessionMode(ctx, tx, binding.scope, &binding, false); err != nil {
		return intake.CSVRowRange{}, err
	}
	taskID, err := k.requireProductTaskWorking(ctx, tx, binding)
	if err != nil {
		return intake.CSVRowRange{}, err
	}
	if taskID != binding.task {
		return intake.CSVRowRange{}, core.Denied
	}
	var storedManifestDigest string
	var encoded []byte
	if err = tx.QueryRow(ctx, `SELECT manifest_digest,manifest FROM task_input_manifests WHERE company_id=$1 AND task_id=$2`, binding.scope.company, taskID).Scan(&storedManifestDigest, &encoded); errors.Is(err, pgx.ErrNoRows) {
		return intake.CSVRowRange{}, core.OutOfScope
	} else if err != nil {
		return intake.CSVRowRange{}, err
	}
	var manifest intake.ModelInputManifest
	if json.Unmarshal(encoded, &manifest) != nil || storedManifestDigest != manifestDigest || manifest.TaskID != taskID ||
		manifest.CompanyID != binding.scope.company || intake.VerifyModelInputManifest(manifest, manifestDigest) != nil {
		return intake.CSVRowRange{}, core.Conflict
	}
	var reference intake.ModelInputManifestEntry
	found := false
	for _, candidate := range manifest.CandidateInputs {
		if candidate.InputID == inputID {
			reference, found = candidate, true
			break
		}
	}
	if !found || reference.Revision != revision || reference.ContentDigest != sourceDigest || !intake.ProviderCSVInputEligible(reference) {
		return intake.CSVRowRange{}, core.OutOfScope
	}
	content, err := readBlobBounded(k.root, binding.scope.company, reference.ContentDigest, intake.MaxUploadBytes)
	if err != nil || int64(len(content)) != reference.ByteSize {
		return intake.CSVRowRange{}, core.Integrity
	}
	result, err := intake.ReadCSVRowRange(reference, manifestDigest, content, startRow, maxRows)
	if err != nil {
		return intake.CSVRowRange{}, core.Malformed
	}
	result.ReadReference = newID()
	if err = appendEvent(ctx, tx, binding.scope, "worker.task_input.csv_range_read", map[string]any{
		"readReference": result.ReadReference, "taskId": taskID, "inputId": reference.InputID,
		"revision": reference.Revision, "manifestDigest": manifestDigest, "sourceDigest": sourceDigest,
		"startRow": result.StartRow, "returnedRows": result.ReturnedRows, "nextRow": result.NextRow,
		"truncated": result.Truncated, "rangeDigest": result.RangeDigest,
		"employeeId": binding.employee, "sessionId": binding.session, "epoch": binding.epoch,
	}); err != nil {
		return intake.CSVRowRange{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return intake.CSVRowRange{}, err
	}
	return result, nil
}
