package installationauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	OwnerSessionCookieName = "polis_owner_session"
	OwnerCSRFCookieName    = "polis_owner_csrf"
	OwnerCSRFHeaderName    = "X-Polis-CSRF-Token"
	ownerSessionLifetime   = 12 * time.Hour
	ownerLoginWindow       = 15 * time.Minute
	ownerLoginBlock        = 15 * time.Minute
	ownerLoginMaxFailures  = 5
)

var (
	ErrOwnerNotInitialized     = errors.New("installation owner is not initialized")
	ErrInvalidOwnerCredentials = errors.New("installation owner credentials are invalid")
	ErrOwnerLoginRateLimited   = errors.New("installation owner login is temporarily rate limited")
)

type LoginResult struct {
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type sessionContextKey struct{}

type authenticatedOwnerSession struct {
	csrfSHA256 string
	expiresAt  time.Time
}

func IsAuthenticated(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(sessionContextKey{}).(authenticatedOwnerSession)
	return ok
}

func SessionExpiry(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	session, ok := ctx.Value(sessionContextKey{}).(authenticatedOwnerSession)
	return session.expiresAt, ok
}

func CSRFValid(ctx context.Context, cookieToken, headerToken string) bool {
	if ctx == nil || cookieToken == "" || headerToken == "" || len(cookieToken) > 128 || len(headerToken) > 128 {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
		return false
	}
	session, ok := ctx.Value(sessionContextKey{}).(authenticatedOwnerSession)
	if !ok {
		return false
	}
	digest := sha256.Sum256([]byte(headerToken))
	return subtle.ConstantTimeCompare([]byte(session.csrfSHA256), []byte(hex.EncodeToString(digest[:]))) == 1
}

func (s *Store) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if s != nil && request != nil {
			if cookie, err := request.Cookie(OwnerSessionCookieName); err == nil && cookie.Value != "" {
				if csrfHash, expiresAt, valid := s.ValidateSession(request.Context(), cookie.Value); valid {
					ctx := context.WithValue(request.Context(), sessionContextKey{}, authenticatedOwnerSession{csrfSHA256: csrfHash, expiresAt: expiresAt})
					request = request.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(response, request)
	})
}

func (s *Store) Login(ctx context.Context, password []byte) (LoginResult, error) {
	if s == nil || s.pool == nil || ctx == nil || len(password) == 0 || len(password) > maxPasswordBytes {
		return LoginResult{}, ErrInvalidOwnerCredentials
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	if err = lockOwnerBootstrap(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return LoginResult{}, err
	}
	if _, err = loadOwnerLoginAttempts(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return LoginResult{}, err
	}
	var passwordHash string
	var passwordRevision int64
	err = tx.QueryRow(ctx, "SELECT password_hash,revision FROM installation_owner WHERE singleton FOR SHARE").Scan(&passwordHash, &passwordRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return LoginResult{}, ErrOwnerNotInitialized
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return LoginResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return LoginResult{}, err
	}
	passwordValid := VerifyPassword(passwordHash, password)

	sessionToken := rand.Text()
	csrfToken := rand.Text()
	sessionDigest := sha256.Sum256([]byte(sessionToken))
	csrfDigest := sha256.Sum256([]byte(csrfToken))
	sessionHash := hex.EncodeToString(sessionDigest[:])
	csrfHash := hex.EncodeToString(csrfDigest[:])

	tx, err = s.pool.Begin(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockOwnerBootstrap(ctx, tx); err != nil {
		return LoginResult{}, err
	}
	if _, err = loadOwnerLoginAttempts(ctx, tx); err != nil {
		return LoginResult{}, err
	}
	var currentHash string
	var currentRevision int64
	err = tx.QueryRow(ctx, "SELECT password_hash,revision FROM installation_owner WHERE singleton FOR SHARE").Scan(&currentHash, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, ErrOwnerNotInitialized
	}
	if err != nil {
		return LoginResult{}, err
	}
	if currentRevision != passwordRevision || currentHash != passwordHash {
		return LoginResult{}, ErrInvalidOwnerCredentials
	}
	if !passwordValid {
		if err = recordOwnerLoginFailure(ctx, tx); err != nil {
			return LoginResult{}, err
		}
		if err = appendOwnerAuthEvent(ctx, tx, "login_failed", nil, nil); err != nil {
			return LoginResult{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{}, ErrInvalidOwnerCredentials
	}
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `INSERT INTO installation_owner_sessions(token_sha256,csrf_sha256,created_at,expires_at)
VALUES($1,$2,clock_timestamp(),clock_timestamp()+($3 * interval '1 second')) RETURNING expires_at`, sessionHash, csrfHash, int64(ownerSessionLifetime/time.Second)).Scan(&expiresAt)
	if err != nil {
		return LoginResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE installation_owner_login_attempts SET window_started_at=clock_timestamp(),failure_count=0,blocked_until=NULL WHERE singleton`); err != nil {
		return LoginResult{}, err
	}
	if err = appendOwnerAuthEvent(ctx, tx, "login_succeeded", nil, &sessionHash); err != nil {
		return LoginResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{SessionToken: sessionToken, CSRFToken: csrfToken, ExpiresAt: expiresAt.UTC()}, nil
}

func (s *Store) ValidateSession(ctx context.Context, sessionToken string) (csrfSHA256 string, expiresAt time.Time, valid bool) {
	if s == nil || s.pool == nil || ctx == nil || sessionToken == "" || len(sessionToken) > 128 {
		return "", time.Time{}, false
	}
	digest := sha256.Sum256([]byte(sessionToken))
	err := s.pool.QueryRow(ctx, `SELECT csrf_sha256,expires_at FROM installation_owner_sessions
WHERE token_sha256=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, hex.EncodeToString(digest[:])).Scan(&csrfSHA256, &expiresAt)
	if err != nil {
		return "", time.Time{}, false
	}
	return csrfSHA256, expiresAt.UTC(), true
}

func (s *Store) RevokeSession(ctx context.Context, sessionToken string) error {
	if s == nil || s.pool == nil || ctx == nil || sessionToken == "" || len(sessionToken) > 128 {
		return nil
	}
	digest := sha256.Sum256([]byte(sessionToken))
	sessionHash := hex.EncodeToString(digest[:])
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockOwnerBootstrap(ctx, tx); err != nil {
		return err
	}
	var revoked bool
	err = tx.QueryRow(ctx, `UPDATE installation_owner_sessions SET revoked_at=clock_timestamp()
WHERE token_sha256=$1 AND revoked_at IS NULL RETURNING true`, sessionHash).Scan(&revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if revoked {
		if err = appendOwnerAuthEvent(ctx, tx, "session_revoked", nil, &sessionHash); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func loadOwnerLoginAttempts(ctx context.Context, tx pgx.Tx) (int, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO installation_owner_login_attempts(singleton) VALUES(true) ON CONFLICT(singleton) DO NOTHING`); err != nil {
		return 0, err
	}
	var failures int
	var windowStarted time.Time
	var blockedUntil *time.Time
	if err := tx.QueryRow(ctx, `SELECT failure_count,window_started_at,blocked_until FROM installation_owner_login_attempts WHERE singleton FOR UPDATE`).Scan(&failures, &windowStarted, &blockedUntil); err != nil {
		return 0, err
	}
	var now time.Time
	if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return 0, err
	}
	if blockedUntil != nil && now.Before(*blockedUntil) {
		return 0, ErrOwnerLoginRateLimited
	}
	if now.Sub(windowStarted) >= ownerLoginWindow || blockedUntil != nil {
		failures = 0
		if _, err := tx.Exec(ctx, `UPDATE installation_owner_login_attempts SET window_started_at=clock_timestamp(),failure_count=0,blocked_until=NULL WHERE singleton`); err != nil {
			return 0, err
		}
	}
	return failures, nil
}

func recordOwnerLoginFailure(ctx context.Context, tx pgx.Tx) error {
	failures, err := loadOwnerLoginAttempts(ctx, tx)
	if err != nil {
		return err
	}
	next := failures + 1
	_, err = tx.Exec(ctx, `UPDATE installation_owner_login_attempts
SET failure_count=$1,blocked_until=CASE WHEN $1 >= $2 THEN clock_timestamp()+($3 * interval '1 second') ELSE NULL END
WHERE singleton`, next, ownerLoginMaxFailures, int64(ownerLoginBlock/time.Second))
	return err
}

func (s *Store) ActiveSessionCount(ctx context.Context) (int64, error) {
	if s == nil || s.pool == nil || ctx == nil {
		return 0, ErrOwnerNotInitialized
	}
	var count int64
	err := s.pool.QueryRow(ctx, `SELECT count(*)::bigint FROM installation_owner_sessions WHERE revoked_at IS NULL AND expires_at>clock_timestamp()`).Scan(&count)
	return count, err
}
