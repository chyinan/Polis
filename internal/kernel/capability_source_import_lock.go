// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
)

type capabilitySourceRequestLockContextKey struct{}

type capabilitySourceRequestLockIdentity struct {
	companyID string
	requestID string
}

type capabilityMCPServerLockContextKey struct{}

type capabilityMCPServerLockIdentity struct {
	companyID string
	serverID  string
}

// lockCapabilitySourceImport coordinates all CAS-first capability imports by
// the receipt key before it takes the package-revision key, on one PG session.
func (k *Kernel) lockCapabilitySourceImport(ctx context.Context, companyID, requestID, publisherScope, packageID, revision string) (func(), error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) || !core.ValidID(publisherScope) || !core.ValidID(packageID) || !validReadOnlySkillRevision(revision) {
		return nil, core.Malformed
	}
	requestKey := capabilitySourceAdvisoryLockKey(companyID, "capability-source-request", requestID)
	revisionKey := capabilitySourceAdvisoryLockKey(companyID, publisherScope, packageID, revision)
	keys := []int64{requestKey}
	if publisherScope == "mcp" {
		keys = appendDistinctAdvisoryLockKey(keys, capabilityMCPServerAdvisoryLockKey(companyID, packageID))
	}
	if revisionKey != requestKey {
		keys = appendDistinctAdvisoryLockKey(keys, revisionKey)
	}
	connection, err := k.lockPool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	acquired := make([]int64, 0, len(keys))
	for _, key := range keys {
		var locked bool
		if err = connection.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
			discardCapabilitySourceLockConnection(connection)
			return nil, err
		}
		if !locked {
			releaseCapabilitySourceLockKeys(connection, acquired)
			return nil, core.Conflict
		}
		acquired = append(acquired, key)
	}
	return func() { releaseCapabilitySourceLockKeys(connection, acquired) }, nil
}

func (k *Kernel) LockStdioMCPPackageServer(ctx context.Context, companyID, serverID string) (context.Context, func(), error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(serverID) {
		return nil, nil, core.Malformed
	}
	connection, err := k.lockPool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	key := capabilityMCPServerAdvisoryLockKey(companyID, serverID)
	var locked bool
	if err = connection.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		discardCapabilitySourceLockConnection(connection)
		return nil, nil, err
	}
	if !locked {
		connection.Release()
		return nil, nil, core.Conflict
	}
	lockedContext := withCapabilityMCPServerLock(ctx, companyID, serverID)
	return lockedContext, func() { releaseCapabilitySourceLockKeys(connection, []int64{key}) }, nil
}

func lockCapabilityMCPServerInTransaction(ctx context.Context, tx pgx.Tx, companyID, serverID string) error {
	if capabilityMCPServerLockHeld(ctx, companyID, serverID) {
		return nil
	}
	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", capabilityMCPServerAdvisoryLockKey(companyID, serverID)).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return core.Conflict
	}
	return nil
}

func capabilitySourceAdvisoryLockKey(companyID, scope string, components ...string) int64 {
	identity := companyID + "\x00" + scope
	for _, component := range components {
		identity += "\x00" + component
	}
	digest := sha256.Sum256([]byte(identity))
	return int64(binary.BigEndian.Uint64(digest[:8]))
}

func capabilityMCPServerAdvisoryLockKey(companyID, serverID string) int64 {
	return capabilitySourceAdvisoryLockKey(companyID, "mcp-package-server", serverID)
}

func appendDistinctAdvisoryLockKey(keys []int64, next int64) []int64 {
	for _, existing := range keys {
		if existing == next {
			return keys
		}
	}
	return append(keys, next)
}

func withCapabilitySourceRequestLock(ctx context.Context, companyID, requestID string) context.Context {
	return context.WithValue(ctx, capabilitySourceRequestLockContextKey{}, capabilitySourceRequestLockIdentity{companyID: companyID, requestID: requestID})
}

func capabilitySourceRequestLockHeld(ctx context.Context, companyID, requestID string) bool {
	identity, ok := ctx.Value(capabilitySourceRequestLockContextKey{}).(capabilitySourceRequestLockIdentity)
	return ok && identity.companyID == companyID && identity.requestID == requestID
}

func withCapabilityMCPServerLock(ctx context.Context, companyID, serverID string) context.Context {
	return context.WithValue(ctx, capabilityMCPServerLockContextKey{}, capabilityMCPServerLockIdentity{companyID: companyID, serverID: serverID})
}

func capabilityMCPServerLockHeld(ctx context.Context, companyID, serverID string) bool {
	identity, ok := ctx.Value(capabilityMCPServerLockContextKey{}).(capabilityMCPServerLockIdentity)
	return ok && identity.companyID == companyID && identity.serverID == serverID
}

func releaseCapabilitySourceLockKeys(connection *pgxpool.Conn, keys []int64) {
	unlockContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for index := len(keys) - 1; index >= 0; index-- {
		var unlocked bool
		if err := connection.QueryRow(unlockContext, "SELECT pg_advisory_unlock($1)", keys[index]).Scan(&unlocked); err != nil || !unlocked {
			discardCapabilitySourceLockConnection(connection)
			return
		}
	}
	connection.Release()
}

func discardCapabilitySourceLockConnection(connection *pgxpool.Conn) {
	conn := connection.Hijack()
	closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Close(closeContext)
}
