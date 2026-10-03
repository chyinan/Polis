// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"polis/internal/core"
	"polis/internal/dbgen"
	"polis/internal/environment"
	"polis/internal/taskvalidation"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Scope struct{ company string }
type Binding struct {
	scope                       Scope
	task                        string
	employee, incarnation       string
	epoch                       int64
	session                     string
	workspaceDigest             string
	workspaceRevision           int64
	taskValidationBindingDigest string
}
type Receipt struct {
	ID                 string `json:"id"`
	Status             string `json:"status"`
	Revision           int64  `json:"revision,omitempty"`
	RuntimeIncarnation string `json:"runtime_incarnation,omitempty"`
}
type Task struct {
	ID, Mission, Owner, State string
	ParentTaskID              string `json:"parent_task_id,omitempty"`
	ProblemKey                string `json:"problem_key"`
	Kind                      core.TaskKind
	Generation                int64
	Plan                      json.RawMessage
	ValidationBinding         *taskvalidation.Binding `json:"validation_binding,omitempty"`
}
type Obligation struct{ ID, State string }
type Artifact struct{ ID, Digest, State, Verdict string }
type Snapshot struct {
	MissionState string
	CompanySeq   int64
	Tasks        []Task
	Employees    []string
	Messages     []string
	Obligations  []Obligation
	Artifacts    []Artifact
	FakeClaims   int64
}
type Kernel struct {
	pool                     *pgxpool.Pool
	lockPool                 *pgxpool.Pool
	lease                    *pgxpool.Conn
	leaseMu                  sync.Mutex
	incarnation              string
	sourceIncarnation        string
	root                     string
	memoryRevocationRoot     string
	memoryRevocationMu       sync.RWMutex
	memoryRevocations        map[string]MemoryRecordRevocationOverlay
	windowsNodeWorkspaceRoot string
	runtimeCASBinding        *RuntimeCASBinding
	executorFingerprints     map[string]environment.EnvironmentExecutorFingerprint
}

// Open never migrates schema. Only in-process fake work can be recovered in R0.
func Open(ctx context.Context, dsn, root string) (*Kernel, error) {
	return openKernel(ctx, dsn, root, nil)
}

// OpenWithRuntimeCASBinding is the activation path. Its recovery phase checks
// every persisted artifact blob before txRecover may mutate control state.
func OpenWithRuntimeCASBinding(ctx context.Context, dsn string, binding RuntimeCASBinding) (*Kernel, error) {
	if err := binding.ValidateSyntax(); err != nil {
		return nil, err
	}
	return openKernel(ctx, dsn, binding.CanonicalRoot, &binding)
}

func openKernel(ctx context.Context, dsn, root string, binding *RuntimeCASBinding) (*Kernel, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "polis_r0_") {
		return nil, core.Denied
	}
	cfg.MaxConns = 8
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	lockCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		p.Close()
		return nil, err
	}
	lockCfg.MaxConns = 2
	lockPool, err := pgxpool.NewWithConfig(ctx, lockCfg)
	if err != nil {
		p.Close()
		return nil, err
	}
	k := &Kernel{pool: p, lockPool: lockPool, incarnation: newID(), root: root, windowsNodeWorkspaceRoot: os.Getenv("POLIS_WINDOWS_NODE_WORKSPACE_ROOT"), runtimeCASBinding: binding, executorFingerprints: make(map[string]environment.EnvironmentExecutorFingerprint)}
	for _, profileID := range []string{environment.WindowsNodeNPMProfile, environment.LinuxNodeNPMProfile} {
		fingerprint, fingerprintErr := environment.CurrentEnvironmentExecutorFingerprint(profileID)
		if fingerprintErr == nil {
			k.executorFingerprints[profileID] = fingerprint
		}
	}
	fail := func(e error) (*Kernel, error) { k.Close(); return nil, e }
	k.lease, err = p.Acquire(ctx)
	if err != nil {
		return fail(err)
	}
	var locked bool
	err = k.lease.QueryRow(ctx, "SELECT pg_try_advisory_lock(714209831)").Scan(&locked)
	if err != nil {
		return fail(err)
	}
	if !locked {
		return fail(core.Denied)
	}
	var version int
	var super bool
	err = p.QueryRow(ctx, "SELECT current_setting('server_version_num')::int,rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&version, &super)
	if err != nil {
		return fail(err)
	}
	if version < 180000 || version >= 190000 || super {
		return fail(core.Denied)
	}
	if err = p.QueryRow(ctx, "SELECT incarnation FROM runtime_control WHERE singleton").Scan(&k.sourceIncarnation); err != nil {
		return fail(err)
	}
	var schema int64
	err = p.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&schema)
	if err != nil {
		return fail(err)
	}
	if schema < 87 {
		return fail(fmt.Errorf("incompatible schema: %d", schema))
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return fail(err)
	}
	if err = k.initializeMemoryRevocationOverlays(); err != nil {
		return fail(fmt.Errorf("initialize memory revocation overlays: %w", err))
	}
	if binding != nil {
		err = k.txRecoverWithRuntimeCASBinding(ctx)
	} else {
		err = k.txRecover(ctx)
	}
	if err != nil {
		return fail(err)
	}
	if err = k.reconcileMemoryRevocationOverlays(ctx); err != nil {
		return fail(fmt.Errorf("apply memory revocation overlays: %w", err))
	}
	if err = k.reconcileEmployeeSchedulesOnStartup(ctx); err != nil {
		return fail(fmt.Errorf("reconcile employee schedules after restart: %w", err))
	}
	return k, nil
}

func (k *Kernel) CurrentEnvironmentExecutorFingerprint(profileID string) (environment.EnvironmentExecutorFingerprint, bool) {
	if k == nil {
		return environment.EnvironmentExecutorFingerprint{}, false
	}
	fingerprint, ok := k.executorFingerprints[profileID]
	if profileID == environment.WindowsNodeNPMProfile && runtime.GOOS == "windows" {
		if !ok {
			return environment.EnvironmentExecutorFingerprint{}, false
		}
		storagePolicyFingerprint, err := environment.CurrentWindowsNodeWorkspacePolicyFingerprint(k.windowsNodeWorkspaceRoot)
		if err != nil {
			return environment.EnvironmentExecutorFingerprint{}, false
		}
		fingerprint.IsolationPolicySHA256 = storagePolicyFingerprint
		return fingerprint, true
	}
	return fingerprint, ok
}
func (k *Kernel) Close() {
	if k == nil {
		return
	}
	k.leaseMu.Lock()
	if k.lease != nil {
		_ = k.lease.Conn().Close(context.Background())
		k.lease.Release()
		k.lease = nil
	}
	k.leaseMu.Unlock()
	if k.pool != nil {
		k.pool.Close()
	}
	if k.lockPool != nil {
		k.lockPool.Close()
	}
}

// LocalScope is a trusted local OS management entry, never offered to workers.
func (k *Kernel) LocalScope(id string) Scope { return Scope{id} }

// Incarnation returns the runtime incarnation bound to this kernel instance.
// Callers use it for evidence identity; worker authorization still validates
// the incarnation inside the guarded database transaction.
func (k *Kernel) Incarnation() string { return k.incarnation }

// SourceIncarnation returns the runtime incarnation observed before recovery
// advanced runtime_control for this kernel instance. It is evidence only.
func (k *Kernel) SourceIncarnation() string { return k.sourceIncarnation }

func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func fingerprint(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func (k *Kernel) guard(ctx context.Context, tx pgx.Tx, s Scope, b *Binding) error {
	return k.guardWithSessionMode(ctx, tx, s, b, true)
}

func (k *Kernel) guardWithSessionMode(ctx context.Context, tx pgx.Tx, s Scope, b *Binding, sessionWrite bool) error {
	if !core.ValidID(s.company) {
		return core.Malformed
	}
	if e := k.checkRuntimeLease(ctx, tx); e != nil {
		return e
	}
	if _, e := dbgen.New(tx).LockCompany(ctx, s.company); errors.Is(e, pgx.ErrNoRows) {
		return core.OutOfScope
	} else if e != nil {
		return e
	}
	if b != nil {
		if e := k.validateBindingGuardWithSessionMode(ctx, tx, s, b, sessionWrite); e != nil {
			return e
		}
	}
	return nil
}

func (k *Kernel) CheckRuntimeLease(ctx context.Context) error {
	if k == nil || k.pool == nil {
		return core.StaleEpoch
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return k.checkRuntimeLease(ctx, tx)
}

func (k *Kernel) checkRuntimeLease(ctx context.Context, tx pgx.Tx) error {
	if k == nil {
		return core.StaleEpoch
	}
	k.leaseMu.Lock()
	alive := k.lease != nil
	var e error
	if alive {
		e = k.lease.Ping(ctx)
	}
	k.leaseMu.Unlock()
	if !alive || e != nil {
		return core.StaleEpoch
	}
	var incarnation string
	if e = tx.QueryRow(ctx, "SELECT incarnation FROM runtime_control WHERE singleton FOR SHARE").Scan(&incarnation); e != nil {
		return e
	}
	if incarnation != k.incarnation {
		return core.StaleEpoch
	}
	return nil
}

func (k *Kernel) validateBindingGuard(ctx context.Context, tx pgx.Tx, s Scope, b *Binding) error {
	return k.validateBindingGuardWithSessionMode(ctx, tx, s, b, true)
}

func (k *Kernel) validateBindingGuardWithSessionMode(ctx context.Context, tx pgx.Tx, s Scope, b *Binding, sessionWrite bool) error {
	if b.scope != s || b.incarnation != k.incarnation {
		return core.StaleEpoch
	}
	var epoch int64
	err := tx.QueryRow(ctx, "SELECT epoch FROM employees WHERE company_id=$1 AND id=$2", s.company, b.employee).Scan(&epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if epoch != b.epoch {
		return core.StaleEpoch
	}
	if b.session != "" {
		if _, err := k.checkSession(ctx, tx, *b, sessionWrite); err != nil {
			return err
		}
	}
	return nil
}

// TXWrite serializes this small slice at the company lifecycle guard. Its
// callback only performs database operations. The event head is advanced last.
func (k *Kernel) TXWrite(ctx context.Context, s Scope, b *Binding, key, op string, input any, fn func(pgx.Tx) (Receipt, error)) (Receipt, error) {
	return k.txWrite(ctx, s, b, key, op, input, true, fn)
}

func (k *Kernel) txWrite(ctx context.Context, s Scope, b *Binding, key, op string, input any, sessionWrite bool, fn func(pgx.Tx) (Receipt, error)) (Receipt, error) {
	if !core.ValidID(key) {
		return Receipt{}, core.Malformed
	}
	actor := "local-owner"
	if b != nil {
		actor = b.employee
	}
	hash := fingerprint(struct {
		Op    string
		Input any
	}{op, input})
	tx, e := k.pool.Begin(ctx)
	if e != nil {
		return Receipt{}, e
	}
	defer tx.Rollback(ctx)
	if !capabilitySourceRequestLockHeld(ctx, s.company, key) {
		requestLockKey := capabilitySourceAdvisoryLockKey(s.company, "capability-source-request", key)
		var locked bool
		if e = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", requestLockKey).Scan(&locked); e != nil {
			return Receipt{}, e
		}
		if !locked {
			// Do not hold a main-pool transaction while a CAS import owns this ID.
			_ = tx.Rollback(context.Background())
			var existingFingerprint string
			var raw []byte
			replayErr := k.pool.QueryRow(ctx, "SELECT fingerprint,result FROM receipts WHERE company_id=$1 AND actor=$2 AND key=$3", s.company, actor, key).Scan(&existingFingerprint, &raw)
			if replayErr == nil {
				if existingFingerprint != hash {
					return Receipt{}, core.Conflict
				}
				var receipt Receipt
				replayErr = json.Unmarshal(raw, &receipt)
				return receipt, replayErr
			}
			if errors.Is(replayErr, pgx.ErrNoRows) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, replayErr
		}
	}
	if e = k.guardWithSessionMode(ctx, tx, s, b, sessionWrite); e != nil {
		return Receipt{}, e
	}
	var old string
	var raw []byte
	e = tx.QueryRow(ctx, "SELECT fingerprint,result FROM receipts WHERE company_id=$1 AND actor=$2 AND key=$3", s.company, actor, key).Scan(&old, &raw)
	if e == nil {
		if old != hash {
			return Receipt{}, core.Conflict
		}
		var r Receipt
		e = json.Unmarshal(raw, &r)
		return r, e
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Receipt{}, e
	}
	r, e := fn(tx)
	if e != nil {
		return Receipt{}, e
	}
	raw, e = json.Marshal(r)
	if e != nil {
		return Receipt{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO receipts VALUES($1,$2,$3,$4,$5)", s.company, actor, key, hash, raw); e != nil {
		return Receipt{}, e
	}
	eventKind := op
	if r.Status == "budget_rejected" {
		eventKind = "problem.budget.route_rejected"
	}
	if e = appendEvent(ctx, tx, s, eventKind, r); e != nil {
		return Receipt{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Receipt{}, e
	}
	return r, nil
}
func appendEvent(ctx context.Context, tx pgx.Tx, s Scope, kind string, payload any) error {
	var seq int64
	if e := tx.QueryRow(ctx, "UPDATE companies SET company_seq=company_seq+1 WHERE id=$1 RETURNING company_seq", s.company).Scan(&seq); e != nil {
		return e
	}
	raw, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		object["occurred_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		raw, e = json.Marshal(object)
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, "INSERT INTO events(company_id,company_seq,kind,payload) VALUES($1,$2,$3,$4)", s.company, seq, kind, raw)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "SELECT pg_notify('polis_company_events',$1)", s.company)
	if notificationEventKind(kind) != "" {
		_ = appendNotificationIntent(ctx, tx, s, notificationEventKind(kind), raw)
	}
	return e
}
