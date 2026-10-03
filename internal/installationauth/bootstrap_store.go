package installationauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	bootstrapLockKey       int64 = 714209843
	bootstrapLifetime            = 10 * time.Minute
	bootstrapAttemptWindow       = 15 * time.Minute
	bootstrapBlockDuration       = 15 * time.Minute
	bootstrapMaxFailures         = 5
	minOwnerPasswordBytes        = 14
)

var (
	ErrOwnerAlreadyInitialized = errors.New("installation owner is already initialized")
	ErrBootstrapUnavailable    = errors.New("installation bootstrap code is invalid or expired")
	ErrBootstrapActive         = errors.New("an unexpired installation bootstrap code already exists")
	ErrBootstrapRateLimited    = errors.New("installation bootstrap is temporarily rate limited")
	ErrOwnerPasswordPolicy     = errors.New("installation owner password does not meet the configured policy")
)

type Store struct {
	pool  *pgxpool.Pool
	owned bool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func OpenStore(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || cfg.ConnConfig.Database == "" {
		return nil, errors.New("installation owner database configuration is invalid")
	}
	if len(cfg.ConnConfig.Database) < len("polis_r0_") || cfg.ConnConfig.Database[:len("polis_r0_")] != "polis_r0_" {
		return nil, errors.New("installation owner store refuses an unscoped database")
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("installation owner database is unavailable")
	}
	store := &Store{pool: pool, owned: true}
	if err = pool.Ping(ctx); err != nil {
		store.Close()
		return nil, errors.New("installation owner database is unavailable")
	}
	var superuser bool
	if err = pool.QueryRow(ctx, "SELECT rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&superuser); err != nil || superuser {
		store.Close()
		return nil, errors.New("installation owner store requires a non-superuser database role")
	}
	return store, nil
}

func (s *Store) Close() {
	if s != nil && s.owned && s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
}

func (s *Store) IssueBootstrap(ctx context.Context) (string, time.Time, error) {
	if s == nil || s.pool == nil || ctx == nil {
		return "", time.Time{}, ErrBootstrapUnavailable
	}
	code := rand.Text()
	digest := sha256.Sum256([]byte(code))
	digestText := hex.EncodeToString(digest[:])
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockOwnerBootstrap(ctx, tx); err != nil {
		return "", time.Time{}, err
	}
	var initialized bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM installation_owner WHERE singleton)").Scan(&initialized); err != nil {
		return "", time.Time{}, err
	}
	if initialized {
		return "", time.Time{}, ErrOwnerAlreadyInitialized
	}
	var revision int64
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `INSERT INTO installation_owner_bootstrap(singleton,token_sha256,revision,issued_at,expires_at,consumed_at)
VALUES(true,$1,1,clock_timestamp(),clock_timestamp()+interval '10 minutes',NULL)
ON CONFLICT(singleton) DO UPDATE SET token_sha256=EXCLUDED.token_sha256,
 revision=installation_owner_bootstrap.revision+1,issued_at=clock_timestamp(),
 expires_at=clock_timestamp()+interval '10 minutes',consumed_at=NULL
WHERE installation_owner_bootstrap.consumed_at IS NOT NULL OR installation_owner_bootstrap.expires_at<=clock_timestamp()
RETURNING revision,expires_at`, digestText).Scan(&revision, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, ErrBootstrapActive
	}
	if err != nil {
		return "", time.Time{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO installation_owner_bootstrap_attempts(singleton,window_started_at,failure_count,blocked_until)
VALUES(true,clock_timestamp(),0,NULL)
ON CONFLICT(singleton) DO UPDATE SET window_started_at=clock_timestamp(),failure_count=0,blocked_until=NULL`); err != nil {
		return "", time.Time{}, err
	}
	if err = appendOwnerAuthEvent(ctx, tx, "bootstrap_issued", &revision, &digestText); err != nil {
		return "", time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", time.Time{}, err
	}
	return code, expiresAt.UTC(), nil
}

func (s *Store) OwnerInitialized(ctx context.Context) (bool, error) {
	if s == nil || s.pool == nil || ctx == nil {
		return false, ErrBootstrapUnavailable
	}
	var initialized bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM installation_owner WHERE singleton)").Scan(&initialized)
	return initialized, err
}

func (s *Store) InitializeOwner(ctx context.Context, code string, password []byte) error {
	if s == nil || s.pool == nil || ctx == nil {
		return ErrBootstrapUnavailable
	}
	if len(password) < minOwnerPasswordBytes || len(password) > maxPasswordBytes {
		return ErrOwnerPasswordPolicy
	}
	if len(code) > 128 {
		return ErrBootstrapUnavailable
	}
	digest := sha256.Sum256([]byte(code))
	digestText := hex.EncodeToString(digest[:])

	valid, revision, err := s.validateBootstrapAttempt(ctx, digestText)
	if err != nil {
		return err
	}
	if !valid {
		return ErrBootstrapUnavailable
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockOwnerBootstrap(ctx, tx); err != nil {
		return err
	}
	var initialized bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM installation_owner WHERE singleton)").Scan(&initialized); err != nil {
		return err
	}
	if initialized {
		return ErrOwnerAlreadyInitialized
	}
	valid, currentRevision, err := checkBootstrapCode(ctx, tx, digestText)
	if err != nil {
		return err
	}
	if !valid || currentRevision != revision {
		return ErrBootstrapUnavailable
	}
	if _, err = tx.Exec(ctx, `INSERT INTO installation_owner(singleton,password_scheme,password_hash,revision)
VALUES(true,$1,$2,1)`, PasswordHashScheme, passwordHash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE installation_owner_bootstrap SET consumed_at=clock_timestamp() WHERE singleton AND revision=$1 AND consumed_at IS NULL`, revision); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE installation_owner_bootstrap_attempts SET window_started_at=clock_timestamp(),failure_count=0,blocked_until=NULL WHERE singleton`); err != nil {
		return err
	}
	if err = appendOwnerAuthEvent(ctx, tx, "owner_initialized", &revision, &digestText); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) validateBootstrapAttempt(ctx context.Context, digestText string) (bool, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	if err = lockOwnerBootstrap(ctx, tx); err != nil {
		return false, 0, err
	}
	var initialized bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM installation_owner WHERE singleton)").Scan(&initialized); err != nil {
		return false, 0, err
	}
	if initialized {
		return false, 0, ErrOwnerAlreadyInitialized
	}
	failures, windowStarted, blockedUntil, err := loadBootstrapAttempts(ctx, tx)
	if err != nil {
		return false, 0, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return false, 0, err
	}
	if blockedUntil != nil && now.Before(*blockedUntil) {
		return false, 0, ErrBootstrapRateLimited
	}
	if now.Sub(windowStarted) >= bootstrapAttemptWindow || blockedUntil != nil {
		failures = 0
		if _, err = tx.Exec(ctx, `UPDATE installation_owner_bootstrap_attempts SET window_started_at=clock_timestamp(),failure_count=0,blocked_until=NULL WHERE singleton`); err != nil {
			return false, 0, err
		}
	}
	valid, revision, err := checkBootstrapCode(ctx, tx, digestText)
	if err != nil {
		return false, 0, err
	}
	if !valid {
		nextFailures := failures + 1
		var blocked any
		if nextFailures >= bootstrapMaxFailures {
			blocked = time.Now().UTC().Add(bootstrapBlockDuration)
		}
		if _, err = tx.Exec(ctx, `UPDATE installation_owner_bootstrap_attempts
SET failure_count=$1,blocked_until=$2 WHERE singleton`, nextFailures, blocked); err != nil {
			return false, 0, err
		}
		if err = appendOwnerAuthEvent(ctx, tx, "bootstrap_failed", nil, &digestText); err != nil {
			return false, 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return valid, revision, nil
}

func loadBootstrapAttempts(ctx context.Context, tx pgx.Tx) (int, time.Time, *time.Time, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO installation_owner_bootstrap_attempts(singleton) VALUES(true) ON CONFLICT(singleton) DO NOTHING`); err != nil {
		return 0, time.Time{}, nil, err
	}
	var failures int
	var windowStarted time.Time
	var blockedUntil *time.Time
	err := tx.QueryRow(ctx, `SELECT failure_count,window_started_at,blocked_until FROM installation_owner_bootstrap_attempts WHERE singleton FOR UPDATE`).Scan(&failures, &windowStarted, &blockedUntil)
	return failures, windowStarted, blockedUntil, err
}

func checkBootstrapCode(ctx context.Context, tx pgx.Tx, digestText string) (bool, int64, error) {
	var expected string
	var revision int64
	var expiresAt time.Time
	var consumedAt *time.Time
	err := tx.QueryRow(ctx, `SELECT token_sha256,revision,expires_at,consumed_at FROM installation_owner_bootstrap WHERE singleton FOR UPDATE`).Scan(&expected, &revision, &expiresAt, &consumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return false, 0, err
	}
	if consumedAt != nil || !now.Before(expiresAt) || subtle.ConstantTimeCompare([]byte(expected), []byte(digestText)) != 1 {
		return false, revision, nil
	}
	return true, revision, nil
}

func lockOwnerBootstrap(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapLockKey)
	return err
}

func appendOwnerAuthEvent(ctx context.Context, tx pgx.Tx, eventType string, revision *int64, subjectSHA256 *string) error {
	_, err := tx.Exec(ctx, `INSERT INTO installation_owner_auth_events(event_type,bootstrap_revision,subject_sha256)
VALUES($1,$2,$3)`, eventType, revision, subjectSHA256)
	return err
}
