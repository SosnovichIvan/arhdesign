package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *Postgres) FindLoginAccount(ctx context.Context, identifier string) (account.LoginAccount, bool, error) {
	var result account.LoginAccount
	var lastName, middleName, globalRole *string
	query := `
		SELECT users.id::text, users.login, users.email, profile.first_name,
		       profile.last_name, profile.middle_name, role.code, role.name,
		       users.status, users.global_role, users.version, users.security_version,
		       credential.password_hash
		  FROM users
		  JOIN profiles profile ON profile.user_id = users.id
		  JOIN professional_roles role ON role.id = profile.professional_role_id
		  JOIN credentials credential ON credential.user_id = users.id
		 WHERE users.login_normalized = $1`
	if strings.Contains(identifier, "@") {
		query = strings.Replace(query, "users.login_normalized = $1", "users.email_normalized = $1", 1)
	}
	err := repository.pool.QueryRow(ctx, query, identifier).Scan(
		&result.ID, &result.Login, &result.Email, &result.FirstName, &lastName, &middleName,
		&result.ProfessionalRoleCode, &result.ProfessionalRoleName, &result.Status, &globalRole,
		&result.Version, &result.SecurityVersion, &result.PasswordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.LoginAccount{}, false, nil
	}
	if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("find login account: %w", err)
	}
	result.LastName, result.MiddleName, result.GlobalRole = lastName, middleName, globalRole
	return result, true, nil
}

func (repository *Postgres) ResolveAccessSession(ctx context.Context, accessHash []byte, now time.Time) (account.AccountView, bool, error) {
	var result account.AccountView
	var lastName, middleName, globalRole *string
	err := repository.pool.QueryRow(ctx, `
		SELECT users.id::text, users.login, users.email, profile.first_name,
		       profile.last_name, profile.middle_name, role.code, role.name,
		       users.status, users.global_role, users.version
		  FROM sessions session
		  JOIN users ON users.id = session.user_id
		  JOIN profiles profile ON profile.user_id = users.id
		  JOIN professional_roles role ON role.id = profile.professional_role_id
		 WHERE session.access_token_hash = $1
		   AND session.revoked_at IS NULL
		   AND session.idle_expires_at > $2
		   AND session.absolute_expires_at > $2
		   AND users.status = 'active'
		   AND users.security_version = session.captured_security_version
	`, accessHash, now).Scan(
		&result.ID, &result.Login, &result.Email, &result.FirstName, &lastName, &middleName,
		&result.ProfessionalRoleCode, &result.ProfessionalRoleName, &result.Status, &globalRole, &result.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.AccountView{}, false, nil
	}
	if err != nil {
		return account.AccountView{}, false, fmt.Errorf("resolve access session: %w", err)
	}
	result.LastName, result.MiddleName, result.GlobalRole = lastName, middleName, globalRole
	return result, true, nil
}

func (repository *Postgres) CreateSession(ctx context.Context, session account.NewSession) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin login session: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var valid bool
	err = tx.QueryRow(ctx, `SELECT status = 'active' AND security_version = $2 FROM users WHERE id = $1 FOR UPDATE`, session.UserID, session.SecurityVersion).Scan(&valid)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !valid {
		return account.ErrAccountUnavailable
	}
	if err != nil {
		return fmt.Errorf("lock login account: %w", err)
	}
	var familyID string
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&familyID); err != nil {
		return fmt.Errorf("generate session family: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO sessions (user_id, family_id, access_token_hash, refresh_token_hash, csrf_token_hash,
			hash_key_version, captured_security_version, created_at, last_seen_at, idle_expires_at, absolute_expires_at, remember_me)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9, $10, $11)
	`, session.UserID, familyID, session.AccessHash, session.RefreshHash, session.CSRFHash, session.HashKeyVersion, session.SecurityVersion, session.Now, session.AccessExpiresAt, session.RefreshExpiresAt, session.RememberMe); err != nil {
		return fmt.Errorf("insert login session: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET last_login_at = $2, updated_at = $2 WHERE id = $1`, session.UserID, session.Now); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_rate_limit_buckets WHERE policy_code='login.account' AND subject_hash=$1`, session.LoginAccountLimitHash); err != nil {
		return fmt.Errorf("clear successful login rate limit: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type, result, request_id, actor_user_id, session_family_id) VALUES ('auth.login', 'success', $1, $2, $3)`, session.RequestID, session.UserID, familyID); err != nil {
		return fmt.Errorf("append login audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit login session: %w", err)
	}
	return nil
}

func (repository *Postgres) BootstrapSuperAdminSession(ctx context.Context, bootstrap account.BootstrapSession) (account.LoginAccount, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("begin super-admin bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('arhdesign.identity.bootstrap'))`); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("lock super-admin bootstrap: %w", err)
	}
	var existingAdmins int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE status = 'active' AND global_role = 'super_admin'`).Scan(&existingAdmins); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("count super-admins: %w", err)
	}
	if existingAdmins > 0 {
		return account.LoginAccount{}, false, nil
	}

	parameters, err := json.Marshal(bootstrap.PasswordParameters)
	if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("encode bootstrap password parameters: %w", err)
	}
	var userID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE login_normalized = $1 FOR UPDATE`, strings.ToLower(bootstrap.Login)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO users (login, login_normalized, email, email_normalized, status, global_role, email_verified_at)
			VALUES ($1, $2, $3, $4, 'active', 'super_admin', $5) RETURNING id::text
		`, bootstrap.Login, strings.ToLower(bootstrap.Login), bootstrap.Email, strings.ToLower(bootstrap.Email), bootstrap.Session.Now).Scan(&userID)
		if err != nil {
			return account.LoginAccount{}, false, fmt.Errorf("insert bootstrap super-admin: %w", err)
		}
	} else if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("find bootstrap account: %w", err)
	} else {
		if _, err = tx.Exec(ctx, `UPDATE users SET status='active', global_role='super_admin', email_verified_at=COALESCE(email_verified_at,$2), security_version=security_version+1, version=version+1, updated_at=$2 WHERE id=$1`, userID, bootstrap.Session.Now); err != nil {
			return account.LoginAccount{}, false, fmt.Errorf("promote bootstrap account: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO profiles (user_id, professional_role_id, first_name)
		SELECT $1, id, $3 FROM professional_roles WHERE code=$2
		ON CONFLICT (user_id) DO NOTHING
	`, userID, bootstrap.ProfessionalRoleCode, bootstrap.FirstName); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("insert bootstrap profile: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO credentials (user_id, password_hash, algorithm, parameters, hash_version, password_changed_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id) DO UPDATE SET password_hash=EXCLUDED.password_hash, algorithm=EXCLUDED.algorithm,
			parameters=EXCLUDED.parameters, hash_version=EXCLUDED.hash_version, password_changed_at=EXCLUDED.password_changed_at
	`, userID, bootstrap.PasswordHash, bootstrap.PasswordAlgorithm, parameters, bootstrap.PasswordHashVersion, bootstrap.Session.Now); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("store bootstrap credential: %w", err)
	}

	var result account.LoginAccount
	var lastName, middleName, globalRole *string
	err = tx.QueryRow(ctx, `
		SELECT users.id::text, users.login, users.email, profile.first_name, profile.last_name, profile.middle_name,
			role.code, role.name, users.status, users.global_role, users.version, users.security_version, credential.password_hash
		FROM users JOIN profiles profile ON profile.user_id=users.id JOIN professional_roles role ON role.id=profile.professional_role_id
		JOIN credentials credential ON credential.user_id=users.id WHERE users.id=$1
	`, userID).Scan(&result.ID, &result.Login, &result.Email, &result.FirstName, &lastName, &middleName, &result.ProfessionalRoleCode, &result.ProfessionalRoleName, &result.Status, &globalRole, &result.Version, &result.SecurityVersion, &result.PasswordHash)
	if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("load bootstrap account: %w", err)
	}
	result.LastName, result.MiddleName, result.GlobalRole = lastName, middleName, globalRole
	bootstrap.Session.UserID, bootstrap.Session.SecurityVersion = userID, result.SecurityVersion
	var familyID string
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&familyID); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("generate bootstrap family: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions (user_id,family_id,access_token_hash,refresh_token_hash,csrf_token_hash,hash_key_version,captured_security_version,created_at,last_seen_at,idle_expires_at,absolute_expires_at,remember_me) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$10,$11)`, userID, familyID, bootstrap.Session.AccessHash, bootstrap.Session.RefreshHash, bootstrap.Session.CSRFHash, bootstrap.Session.HashKeyVersion, result.SecurityVersion, bootstrap.Session.Now, bootstrap.Session.AccessExpiresAt, bootstrap.Session.RefreshExpiresAt, bootstrap.Session.RememberMe); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("insert bootstrap session: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET last_login_at=$2, updated_at=$2 WHERE id=$1`, userID, bootstrap.Session.Now); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("update bootstrap login: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_rate_limit_buckets WHERE policy_code='login.account' AND subject_hash=$1`, bootstrap.Session.LoginAccountLimitHash); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("clear bootstrap login rate limit: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,subject_user_id,session_family_id) VALUES ('account.super_admin_bootstrapped','success',$1,$2,'user',$2,NULL),('auth.login','success',$1,$2,NULL,NULL,$3)`, bootstrap.Session.RequestID, userID, familyID); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("append bootstrap audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("commit super-admin bootstrap: %w", err)
	}
	return result, true, nil
}

func (repository *Postgres) EnsureTechnicalAdmin(ctx context.Context, bootstrap account.TechnicalAdminBootstrap) (account.AccountView, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return account.AccountView{}, false, fmt.Errorf("begin technical-admin bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('arhdesign.identity.technical-admin-bootstrap'))`); err != nil {
		return account.AccountView{}, false, fmt.Errorf("lock technical-admin bootstrap: %w", err)
	}

	var activeID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE status='active' AND global_role='technical_admin' FOR UPDATE`).Scan(&activeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return account.AccountView{}, false, fmt.Errorf("find active technical administrator: %w", err)
	}

	var userID string
	var priorRole *string
	err = tx.QueryRow(ctx, `SELECT id::text,global_role FROM users WHERE login_normalized=$1 FOR UPDATE`, strings.ToLower(bootstrap.Login)).Scan(&userID, &priorRole)
	created := false
	adopted := false
	if errors.Is(err, pgx.ErrNoRows) {
		if activeID != "" {
			userID = activeID
			technicalRole := "technical_admin"
			priorRole = &technicalRole
			adopted = true
		} else {
			err = tx.QueryRow(ctx, `
				INSERT INTO users (login,login_normalized,email,email_normalized,status,global_role,email_verified_at,created_at,updated_at)
				VALUES ($1,$2,$3,$4,'active','technical_admin',$5,$5,$5)
				RETURNING id::text
			`, bootstrap.Login, strings.ToLower(bootstrap.Login), bootstrap.Email, strings.ToLower(bootstrap.Email), bootstrap.Now).Scan(&userID)
			if err != nil {
				return account.AccountView{}, false, mapAccountBootstrapWriteError("insert technical administrator", err)
			}
			created = true
		}
	} else if err != nil {
		return account.AccountView{}, false, fmt.Errorf("find configured technical administrator: %w", err)
	} else {
		if activeID != "" && activeID != userID {
			return account.AccountView{}, false, account.ErrIdentifierUnavailable
		}
		if priorRole != nil && *priorRole != "technical_admin" {
			return account.AccountView{}, false, account.ErrIdentifierUnavailable
		}
	}
	if !created {
		if _, err = tx.Exec(ctx, `
			UPDATE users
			SET login=$3,login_normalized=$4,email=$5,email_normalized=$6,status='active',global_role='technical_admin',
			    email_verified_at=COALESCE(email_verified_at,$2),updated_at=$2,
			    version=CASE WHEN login<>$3 OR email<>$5 OR status<>'active' OR global_role IS DISTINCT FROM 'technical_admin' THEN version+1 ELSE version END
			WHERE id=$1
		`, userID, bootstrap.Now, bootstrap.Login, strings.ToLower(bootstrap.Login), bootstrap.Email, strings.ToLower(bootstrap.Email)); err != nil {
			return account.AccountView{}, false, mapAccountBootstrapWriteError("ensure technical administrator state", err)
		}
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO profiles (user_id,professional_role_id,first_name,updated_at)
		SELECT $1,id,$3,$4 FROM professional_roles WHERE code=$2
		ON CONFLICT (user_id) DO NOTHING
	`, userID, bootstrap.ProfessionalRoleCode, bootstrap.FirstName, bootstrap.Now); err != nil {
		return account.AccountView{}, false, fmt.Errorf("insert technical administrator profile: %w", err)
	}
	if created || adopted || priorRole == nil {
		parameters, marshalErr := json.Marshal(bootstrap.PasswordParameters)
		if marshalErr != nil {
			return account.AccountView{}, false, fmt.Errorf("encode technical administrator password parameters: %w", marshalErr)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO credentials (user_id,password_hash,algorithm,parameters,hash_version,password_changed_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (user_id) DO UPDATE SET password_hash=EXCLUDED.password_hash,algorithm=EXCLUDED.algorithm,
				parameters=EXCLUDED.parameters,hash_version=EXCLUDED.hash_version,password_changed_at=EXCLUDED.password_changed_at
		`, userID, bootstrap.PasswordHash, bootstrap.PasswordAlgorithm, parameters, bootstrap.PasswordHashVersion, bootstrap.Now); err != nil {
			return account.AccountView{}, false, fmt.Errorf("store technical administrator credential: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,subject_user_id,occurred_at)
		VALUES ('account.technical_admin_bootstrapped','success',$1,$2,'user',$2,$3)
	`, "technical-admin-bootstrap", userID, bootstrap.Now); err != nil {
		return account.AccountView{}, false, fmt.Errorf("append technical administrator bootstrap audit: %w", err)
	}

	result, err := loadAccountView(ctx, tx, userID)
	if err != nil {
		return account.AccountView{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return account.AccountView{}, false, fmt.Errorf("commit technical-admin bootstrap: %w", err)
	}
	return result, created, nil
}

func mapAccountBootstrapWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return account.ErrIdentifierUnavailable
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func loadAccountView(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID string) (account.AccountView, error) {
	var result account.AccountView
	var lastName, middleName, globalRole *string
	err := querier.QueryRow(ctx, `
		SELECT users.id::text,users.login,users.email,profile.first_name,profile.last_name,profile.middle_name,
		       role.code,role.name,users.status,users.global_role,users.version
		FROM users
		JOIN profiles profile ON profile.user_id=users.id
		JOIN professional_roles role ON role.id=profile.professional_role_id
		WHERE users.id=$1
	`, userID).Scan(&result.ID, &result.Login, &result.Email, &result.FirstName, &lastName, &middleName,
		&result.ProfessionalRoleCode, &result.ProfessionalRoleName, &result.Status, &globalRole, &result.Version)
	if err != nil {
		return account.AccountView{}, fmt.Errorf("load account view: %w", err)
	}
	result.LastName, result.MiddleName, result.GlobalRole = lastName, middleName, globalRole
	return result, nil
}

func (repository *Postgres) RotateSession(ctx context.Context, rotation account.SessionRotation) (account.LoginAccount, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return account.LoginAccount{}, fmt.Errorf("begin session rotation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sessionID, familyID, userID string
	var storedCSRF []byte
	var revokedAt *time.Time
	var replacedID *string
	var lastSeen, absoluteExpires time.Time
	var capturedVersion int64
	var remember bool
	err = tx.QueryRow(ctx, `SELECT id::text,family_id::text,user_id::text,csrf_token_hash,revoked_at,replaced_by_session_id::text,last_seen_at,absolute_expires_at,captured_security_version,remember_me FROM sessions WHERE refresh_token_hash=$1 FOR UPDATE`, rotation.RefreshHash).Scan(&sessionID, &familyID, &userID, &storedCSRF, &revokedAt, &replacedID, &lastSeen, &absoluteExpires, &capturedVersion, &remember)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.LoginAccount{}, account.ErrUnauthenticated
	}
	if err != nil {
		return account.LoginAccount{}, fmt.Errorf("lock refresh session: %w", err)
	}
	if replacedID != nil {
		if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,$2), revoke_reason=CASE WHEN revoked_at IS NULL THEN 'reuse_detected' ELSE revoke_reason END WHERE family_id=$1`, familyID, rotation.Now); err != nil {
			return account.LoginAccount{}, fmt.Errorf("revoke reused session family: %w", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,reason_code,request_id,actor_user_id,session_family_id) VALUES ('auth.refresh','reuse_detected','reuse_detected',$1,$2,$3)`, rotation.RequestID, userID, familyID); err != nil {
			return account.LoginAccount{}, fmt.Errorf("audit refresh reuse: %w", err)
		}
		if err = tx.Commit(ctx); err != nil {
			return account.LoginAccount{}, fmt.Errorf("commit refresh reuse revoke: %w", err)
		}
		return account.LoginAccount{}, account.ErrUnauthenticated
	}
	if revokedAt != nil || !absoluteExpires.After(rotation.Now) || !lastSeen.Add(30*24*time.Hour).After(rotation.Now) || !bytes.Equal(storedCSRF, rotation.CSRFHash) {
		return account.LoginAccount{}, account.ErrUnauthenticated
	}
	var result account.LoginAccount
	var lastName, middleName, globalRole *string
	err = tx.QueryRow(ctx, `SELECT users.id::text,users.login,users.email,profile.first_name,profile.last_name,profile.middle_name,role.code,role.name,users.status,users.global_role,users.version,users.security_version,credential.password_hash FROM users JOIN profiles profile ON profile.user_id=users.id JOIN professional_roles role ON role.id=profile.professional_role_id JOIN credentials credential ON credential.user_id=users.id WHERE users.id=$1 FOR UPDATE OF users`, userID).Scan(&result.ID, &result.Login, &result.Email, &result.FirstName, &lastName, &middleName, &result.ProfessionalRoleCode, &result.ProfessionalRoleName, &result.Status, &globalRole, &result.Version, &result.SecurityVersion, &result.PasswordHash)
	if err != nil {
		return account.LoginAccount{}, fmt.Errorf("lock refresh account: %w", err)
	}
	if result.Status != "active" || result.SecurityVersion != capturedVersion {
		return account.LoginAccount{}, account.ErrUnauthenticated
	}
	result.LastName, result.MiddleName, result.GlobalRole, result.RememberMe, result.RefreshExpiresAt = lastName, middleName, globalRole, remember, absoluteExpires
	var replacementID string
	err = tx.QueryRow(ctx, `INSERT INTO sessions (user_id,family_id,access_token_hash,refresh_token_hash,csrf_token_hash,hash_key_version,captured_security_version,created_at,last_seen_at,idle_expires_at,absolute_expires_at,remember_me) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$10,$11) RETURNING id::text`, userID, familyID, rotation.NewAccessHash, rotation.NewRefreshHash, rotation.NewCSRFHash, rotation.HashKeyVersion, capturedVersion, rotation.Now, rotation.AccessExpiresAt, absoluteExpires, remember).Scan(&replacementID)
	if err != nil {
		return account.LoginAccount{}, fmt.Errorf("insert rotated session: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=$2,revoke_reason='rotated',replaced_by_session_id=$3 WHERE id=$1`, sessionID, rotation.Now, replacementID); err != nil {
		return account.LoginAccount{}, fmt.Errorf("consume refresh session: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,session_family_id) VALUES ('auth.refresh','success',$1,$2,$3)`, rotation.RequestID, userID, familyID); err != nil {
		return account.LoginAccount{}, fmt.Errorf("audit refresh success: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return account.LoginAccount{}, fmt.Errorf("commit session rotation: %w", err)
	}
	return result, nil
}

func (repository *Postgres) RevokeSessionFamily(ctx context.Context, revocation account.SessionRevocation) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin logout: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var familyID, userID string
	var storedCSRF []byte
	err = tx.QueryRow(ctx, `SELECT family_id::text,user_id::text,csrf_token_hash FROM sessions WHERE access_token_hash=$1 OR refresh_token_hash=$2 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, revocation.AccessHash, revocation.RefreshHash).Scan(&familyID, &userID, &storedCSRF)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find logout session: %w", err)
	}
	if !bytes.Equal(storedCSRF, revocation.CSRFHash) {
		return account.ErrUnauthenticated
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,$2),revoke_reason=CASE WHEN revoked_at IS NULL THEN 'logout' ELSE revoke_reason END WHERE family_id=$1`, familyID, revocation.Now); err != nil {
		return fmt.Errorf("revoke logout family: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,session_family_id) VALUES ('auth.logout','success',$1,$2,$3)`, revocation.RequestID, userID, familyID); err != nil {
		return fmt.Errorf("audit logout: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit logout: %w", err)
	}
	return nil
}

func (repository *Postgres) RecordLoginFailure(ctx context.Context, result, requestID string) error {
	if result != "invalid_credentials" && result != "unverified" && result != "disabled" && result != "rate_limited" {
		return fmt.Errorf("unsupported login failure result")
	}
	if _, err := repository.pool.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id) VALUES ('auth.login',$1,$2)`, result, requestID); err != nil {
		return fmt.Errorf("append rejected login audit: %w", err)
	}
	return nil
}

func (repository *Postgres) ConsumeRateLimit(ctx context.Context, request account.RateLimitRequest) (time.Duration, error) {
	if request.Capacity < 1 || request.FullRefill <= 0 || request.Retention <= 0 || request.HashKeyVersion < 1 || len(request.SubjectHash) == 0 {
		return 0, fmt.Errorf("invalid rate-limit policy")
	}
	refillSeconds := request.FullRefill.Seconds()
	var accepted bool
	err := repository.pool.QueryRow(ctx, `
		WITH consumed AS (
			INSERT INTO auth_rate_limit_buckets (policy_code,subject_hash,hash_key_version,tokens,last_refill_at,blocked_until,expires_at)
			VALUES ($1,$2,$3,$4::numeric-1,$5::timestamptz,NULL,$5::timestamptz+make_interval(secs => $7::double precision))
			ON CONFLICT (policy_code,subject_hash) DO UPDATE SET
				tokens=LEAST($4::numeric,auth_rate_limit_buckets.tokens + GREATEST(0,EXTRACT(EPOCH FROM ($5::timestamptz-auth_rate_limit_buckets.last_refill_at))) * $4::numeric / $6::numeric)-1,
				last_refill_at=$5::timestamptz,blocked_until=NULL,expires_at=$5::timestamptz+make_interval(secs => $7::double precision),hash_key_version=$3
			WHERE (auth_rate_limit_buckets.blocked_until IS NULL OR auth_rate_limit_buckets.blocked_until <= $5::timestamptz)
			  AND LEAST($4::numeric,auth_rate_limit_buckets.tokens + GREATEST(0,EXTRACT(EPOCH FROM ($5::timestamptz-auth_rate_limit_buckets.last_refill_at))) * $4::numeric / $6::numeric) >= 1
			RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM consumed)
	`, request.PolicyCode, request.SubjectHash, request.HashKeyVersion, request.Capacity, request.Now, refillSeconds, request.Retention.Seconds()).Scan(&accepted)
	if err != nil {
		return 0, fmt.Errorf("consume rate-limit bucket: %w", err)
	}
	if accepted {
		return 0, nil
	}
	var tokens float64
	var lastRefill time.Time
	var blockedUntil *time.Time
	err = repository.pool.QueryRow(ctx, `SELECT tokens::double precision,last_refill_at,blocked_until FROM auth_rate_limit_buckets WHERE policy_code=$1 AND subject_hash=$2`, request.PolicyCode, request.SubjectHash).Scan(&tokens, &lastRefill, &blockedUntil)
	if err != nil {
		return 0, fmt.Errorf("load exhausted rate-limit bucket: %w", err)
	}
	if blockedUntil != nil && blockedUntil.After(request.Now) {
		return blockedUntil.Sub(request.Now), nil
	}
	refilled := math.Min(float64(request.Capacity), tokens+math.Max(0, request.Now.Sub(lastRefill).Seconds())*float64(request.Capacity)/refillSeconds)
	seconds := math.Ceil(math.Max(1, 1-refilled) * refillSeconds / float64(request.Capacity))
	return time.Duration(seconds) * time.Second, nil
}

type EmailOutboxEntry struct {
	ID                string
	MessageType       string
	PayloadCiphertext []byte
	PayloadKeyVersion int
	Attempts          int
}

type TelegramAccountOutboxEntry struct {
	ID                string
	ChatID            int64
	MessageType       string
	PayloadCiphertext []byte
	PayloadKeyVersion int
	Attempts          int
}

func (repository *Postgres) CreatePendingAccount(ctx context.Context, pending account.PendingAccount) error {
	parameters, err := json.Marshal(pending.PasswordParameters)
	if err != nil {
		return fmt.Errorf("encode password parameters: %w", err)
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin registration: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var userID string
	err = transaction.QueryRow(ctx, `
		INSERT INTO users (login, login_normalized, email, email_normalized)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text
	`, pending.Login, pending.LoginNormalized, pending.Email, pending.EmailNormalized).Scan(&userID)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return account.ErrIdentifierUnavailable
		}
		return fmt.Errorf("insert pending account: %w", err)
	}

	result, err := transaction.Exec(ctx, `
		INSERT INTO profiles (user_id, professional_role_id, first_name, last_name, middle_name)
		SELECT $1, id, $3, NULLIF($4, ''), NULLIF($5, '')
		  FROM professional_roles
		 WHERE code = $2 AND is_active
	`, userID, pending.ProfessionalRoleCode, pending.FirstName, pending.LastName, pending.MiddleName)
	if err != nil {
		return fmt.Errorf("insert pending profile: %w", err)
	}
	if result.RowsAffected() != 1 {
		return account.ErrInvalidInput
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO credentials (user_id, password_hash, algorithm, parameters, hash_version)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, pending.PasswordHash, pending.PasswordAlgorithm, parameters, pending.PasswordHashVersion); err != nil {
		return fmt.Errorf("insert credential: %w", err)
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO audit_events (event_type, result, request_id, subject_type, subject_user_id)
		VALUES ('account.registration_accepted', 'pending_verification', $1, 'user', $2)
	`, pending.RequestID, userID); err != nil {
		return fmt.Errorf("append registration audit: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit registration: %w", err)
	}
	return nil
}

func (repository *Postgres) SetVerificationChannel(ctx context.Context, selection account.VerificationChannelSelection) (bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin verification channel selection: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `SELECT id::text FROM users WHERE status='pending_verification' AND login_normalized=$1 FOR UPDATE`
	if strings.Contains(selection.IdentifierNormalized, "@") {
		query = `SELECT id::text FROM users WHERE status='pending_verification' AND email_normalized=$1 FOR UPDATE`
	}
	var userID string
	err = tx.QueryRow(ctx, query, selection.IdentifierNormalized).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock pending account for channel selection: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO account_notification_preferences (user_id, verification_channel, updated_at)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id) DO UPDATE
		SET verification_channel=EXCLUDED.verification_channel, updated_at=EXCLUDED.updated_at
	`, userID, selection.Channel, selection.Now); err != nil {
		return false, fmt.Errorf("save verification channel preference: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE email_verification_tokens
		SET revoked_at=$2,revoke_reason='channel_changed'
		WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL
	`, userID, selection.Now); err != nil {
		return false, fmt.Errorf("revoke previous verification delivery: %w", err)
	}
	if selection.Channel == "email" {
		if _, err = tx.Exec(ctx, `
			INSERT INTO email_verification_tokens (user_id,token_hash,hash_key_version,expires_at)
			VALUES ($1,$2,$3,$4)
		`, userID, selection.TokenHash, selection.TokenKeyVersion, selection.TokenExpiresAt); err != nil {
			return false, fmt.Errorf("insert selected email verification token: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version)
			VALUES ($1,'email',$2,$3,$4,$5)
		`, userID, verificationMessageType, selection.OutboxIdempotencyKey, selection.OutboxCiphertext, selection.OutboxKeyVersion); err != nil {
			return false, fmt.Errorf("enqueue selected verification email: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type,result,request_id,subject_type,subject_user_id,metadata)
		VALUES ('account.verification_channel_selected','accepted',$1,'user',$2,jsonb_build_object('channel',$3::text))
	`, selection.RequestID, userID, selection.Channel); err != nil {
		return false, fmt.Errorf("append verification channel audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit verification channel selection: %w", err)
	}
	return true, nil
}

func (repository *Postgres) BeginTelegramAuth(ctx context.Context, start account.TelegramAuthStart) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO telegram_bot_auth_states (chat_id,flow,step,candidate_user_id,request_id,failed_attempts,started_at,expires_at)
		VALUES ($1,$2,'awaiting_login',NULL,$3,0,$4,$5)
		ON CONFLICT (chat_id) DO UPDATE
		SET flow=EXCLUDED.flow,step='awaiting_login',candidate_user_id=NULL,request_id=EXCLUDED.request_id,
		    failed_attempts=0,started_at=EXCLUDED.started_at,expires_at=EXCLUDED.expires_at
	`, start.ChatID, start.Flow, start.RequestID, start.StartedAt, start.ExpiresAt)
	if err != nil {
		return fmt.Errorf("begin Telegram auth dialog: %w", err)
	}
	return nil
}

func (repository *Postgres) AdvanceTelegramAuth(ctx context.Context, advance account.TelegramAuthAdvance) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE telegram_bot_auth_states
		SET step='awaiting_password',candidate_user_id=$2
		WHERE chat_id=$1 AND step='awaiting_login' AND expires_at>$3
	`, advance.ChatID, advance.CandidateUserID, advance.Now)
	if err != nil {
		return fmt.Errorf("advance Telegram auth dialog: %w", err)
	}
	if result.RowsAffected() != 1 {
		return account.ErrInvalidToken
	}
	return nil
}

func (repository *Postgres) TelegramAuthStep(ctx context.Context, chatID int64, now time.Time) (string, error) {
	var step string
	err := repository.pool.QueryRow(ctx, `SELECT step FROM telegram_bot_auth_states WHERE chat_id=$1 AND expires_at>$2`, chatID, now).Scan(&step)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", account.ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("load Telegram auth step: %w", err)
	}
	return step, nil
}

func (repository *Postgres) TelegramAuthAccount(ctx context.Context, chatID int64, now time.Time) (account.LoginAccount, bool, error) {
	var candidateID *string
	err := repository.pool.QueryRow(ctx, `
		SELECT candidate_user_id::text
		FROM telegram_bot_auth_states
		WHERE chat_id=$1 AND step='awaiting_password' AND expires_at>$2
	`, chatID, now).Scan(&candidateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.LoginAccount{}, false, account.ErrInvalidToken
	}
	if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("load Telegram auth dialog: %w", err)
	}
	if candidateID == nil {
		return account.LoginAccount{}, false, nil
	}
	var result account.LoginAccount
	var lastName, middleName, globalRole *string
	err = repository.pool.QueryRow(ctx, `
		SELECT users.id::text,users.login,users.email,profile.first_name,profile.last_name,profile.middle_name,
		       role.code,role.name,users.status,users.global_role,users.version,users.security_version,credential.password_hash
		FROM users
		JOIN profiles profile ON profile.user_id=users.id
		JOIN professional_roles role ON role.id=profile.professional_role_id
		JOIN credentials credential ON credential.user_id=users.id
		WHERE users.id=$1
	`, *candidateID).Scan(&result.ID, &result.Login, &result.Email, &result.FirstName, &lastName, &middleName, &result.ProfessionalRoleCode, &result.ProfessionalRoleName, &result.Status, &globalRole, &result.Version, &result.SecurityVersion, &result.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.LoginAccount{}, false, nil
	}
	if err != nil {
		return account.LoginAccount{}, false, fmt.Errorf("load Telegram auth account: %w", err)
	}
	result.LastName, result.MiddleName, result.GlobalRole = lastName, middleName, globalRole
	return result, true, nil
}

func (repository *Postgres) CompleteTelegramAuth(ctx context.Context, completion account.TelegramAuthCompletion) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram account confirmation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var candidateID, status string
	var globalRole *string
	err = tx.QueryRow(ctx, `
		SELECT state.candidate_user_id::text,users.status,users.global_role
		FROM telegram_bot_auth_states state
		JOIN users ON users.id=state.candidate_user_id
		WHERE state.chat_id=$1 AND state.step='awaiting_password' AND state.expires_at>$2
		FOR UPDATE OF state,users
	`, completion.ChatID, completion.Now).Scan(&candidateID, &status, &globalRole)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && candidateID != completion.UserID || status == "disabled" {
		return account.ErrInvalidToken
	}
	if err != nil {
		return fmt.Errorf("lock Telegram confirmation state: %w", err)
	}
	if status == "pending_verification" {
		if _, err = tx.Exec(ctx, `UPDATE users SET status='active',updated_at=$2,version=version+1 WHERE id=$1`, completion.UserID, completion.Now); err != nil {
			return fmt.Errorf("activate Telegram-confirmed account: %w", err)
		}
		if _, err = tx.Exec(ctx, `UPDATE email_verification_tokens SET revoked_at=$2,revoke_reason='telegram_confirmed' WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, completion.UserID, completion.Now); err != nil {
			return fmt.Errorf("revoke email verification after Telegram confirmation: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE telegram_account_bindings SET revoked_at=$3,revoke_reason='replaced',version=version+1 WHERE revoked_at IS NULL AND (user_id=$1 OR chat_id=$2)`, completion.UserID, completion.ChatID, completion.Now); err != nil {
		return fmt.Errorf("revoke prior Telegram account binding: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO telegram_account_bindings (user_id,chat_id,chat_username,verified_at) VALUES ($1,$2,NULLIF($3,''),$4)`, completion.UserID, completion.ChatID, completion.ChatUsername, completion.Now); err != nil {
		return fmt.Errorf("bind Telegram account chat: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO account_notification_preferences (user_id,verification_channel,recovery_channel,telegram_events_enabled,updated_at)
		VALUES ($1,'telegram','telegram',TRUE,$2)
		ON CONFLICT (user_id) DO UPDATE
		SET verification_channel='telegram',recovery_channel='telegram',telegram_events_enabled=TRUE,updated_at=EXCLUDED.updated_at
	`, completion.UserID, completion.Now); err != nil {
		return fmt.Errorf("save Telegram account preferences: %w", err)
	}
	if globalRole != nil && (*globalRole == "super_admin" || *globalRole == "technical_admin") {
		if _, err = tx.Exec(ctx, `INSERT INTO telegram_subscribers (chat_id,username,user_id,verified_at,subscribed_at,unsubscribed_at,updated_at,version) VALUES ($1,NULLIF($2,''),$3,$4,$4,NULL,$4,1) ON CONFLICT (chat_id) DO UPDATE SET username=EXCLUDED.username,user_id=EXCLUDED.user_id,verified_at=EXCLUDED.verified_at,unsubscribed_at=NULL,updated_at=EXCLUDED.updated_at,version=telegram_subscribers.version+1`, completion.ChatID, completion.ChatUsername, completion.UserID, completion.Now); err != nil {
			return fmt.Errorf("activate privileged Telegram subscription: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM telegram_bot_auth_states WHERE chat_id=$1`, completion.ChatID); err != nil {
		return fmt.Errorf("consume Telegram auth dialog: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,subject_type,subject_user_id,metadata) VALUES ('account.telegram_confirmed','success',$1,'user',$2,jsonb_build_object('was_pending',$3::boolean))`, completion.RequestID, completion.UserID, status == "pending_verification"); err != nil {
		return fmt.Errorf("append Telegram confirmation audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram account confirmation: %w", err)
	}
	return nil
}

func (repository *Postgres) RejectTelegramAuth(ctx context.Context, rejection account.TelegramAuthRejection) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram auth rejection: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM telegram_bot_auth_states WHERE chat_id=$1`, rejection.ChatID); err != nil {
		return fmt.Errorf("clear rejected Telegram auth state: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,reason_code,request_id) VALUES ('account.telegram_confirmed','rejected',$1,$2)`, rejection.Reason, rejection.RequestID); err != nil {
		return fmt.Errorf("append Telegram auth rejection audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram auth rejection: %w", err)
	}
	return nil
}

const verificationMessageType = "account.email_verification"
const passwordResetMessageType = "account.password_reset"

func (repository *Postgres) ConsumeVerificationToken(ctx context.Context, tokenHash []byte, keyVersion int, now time.Time, requestID string) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin email verification: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var tokenID, userID, status string
	var expiresAt time.Time
	err = transaction.QueryRow(ctx, `
		SELECT token.id::text, token.user_id::text, token.expires_at, users.status
		  FROM email_verification_tokens token
		  JOIN users ON users.id = token.user_id
		 WHERE token.token_hash = $1
		   AND token.hash_key_version = $2
		   AND token.consumed_at IS NULL
		   AND token.revoked_at IS NULL
		 FOR UPDATE OF token, users
	`, tokenHash, keyVersion).Scan(&tokenID, &userID, &expiresAt, &status)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!expiresAt.After(now) || status != "pending_verification") {
		_ = transaction.Rollback(ctx)
		if _, auditErr := repository.pool.Exec(ctx, `
			INSERT INTO audit_events (event_type, result, reason_code, request_id)
			VALUES ('account.email_verified', 'invalid_or_expired', 'invalid_or_expired', $1)
		`, requestID); auditErr != nil {
			return fmt.Errorf("append rejected verification audit: %w", auditErr)
		}
		return account.ErrInvalidToken
	}
	if err != nil {
		return fmt.Errorf("load verification token: %w", err)
	}
	if _, err = transaction.Exec(ctx, `UPDATE email_verification_tokens SET consumed_at = $2 WHERE id = $1`, tokenID, now); err != nil {
		return fmt.Errorf("consume verification token: %w", err)
	}
	if _, err = transaction.Exec(ctx, `UPDATE users SET status = 'active', email_verified_at = $2, updated_at = $2, version = version + 1 WHERE id = $1`, userID, now); err != nil {
		return fmt.Errorf("activate account: %w", err)
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO audit_events (event_type, result, request_id, subject_type, subject_user_id)
		VALUES ('account.email_verified', 'success', $1, 'user', $2)
	`, requestID, userID); err != nil {
		return fmt.Errorf("append verification audit: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit email verification: %w", err)
	}
	return nil
}

func (repository *Postgres) PendingAccountEmail(ctx context.Context, identifier string) (string, bool, error) {
	var email string
	query := `
		SELECT email
		  FROM users
		 WHERE status = 'pending_verification'
		   AND login_normalized = $1
	`
	if strings.Contains(identifier, "@") {
		query = `
			SELECT email
			  FROM users
			 WHERE status = 'pending_verification'
			   AND email_normalized = $1
		`
	}
	err := repository.pool.QueryRow(ctx, query, identifier).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve pending account: %w", err)
	}
	return email, true, nil
}

func (repository *Postgres) ReplaceVerificationToken(ctx context.Context, replacement account.ReplacementVerification) (bool, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin verification resend: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var userID string
	query := `
		SELECT id::text
		  FROM users
		 WHERE status = 'pending_verification'
		   AND login_normalized = $1
		 FOR UPDATE
	`
	if strings.Contains(replacement.IdentifierNormalized, "@") {
		query = `
			SELECT id::text
			  FROM users
			 WHERE status = 'pending_verification'
			   AND email_normalized = $1
			 FOR UPDATE
		`
	}
	err = transaction.QueryRow(ctx, query, replacement.IdentifierNormalized).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock pending account: %w", err)
	}
	if _, err = transaction.Exec(ctx, `
		UPDATE email_verification_tokens
		   SET revoked_at = NOW(), revoke_reason = 'resend'
		 WHERE user_id = $1 AND consumed_at IS NULL AND revoked_at IS NULL
	`, userID); err != nil {
		return false, fmt.Errorf("revoke prior verification token: %w", err)
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, hash_key_version, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, replacement.TokenHash, replacement.TokenKeyVersion, replacement.TokenExpiresAt); err != nil {
		return false, fmt.Errorf("insert replacement verification token: %w", err)
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id, channel, message_type, idempotency_key,
			payload_ciphertext, payload_key_version
		) VALUES ($1, 'email', $2, $3, $4, $5)
	`, userID, verificationMessageType, replacement.OutboxIdempotencyKey, replacement.OutboxCiphertext, replacement.OutboxKeyVersion); err != nil {
		return false, fmt.Errorf("enqueue replacement verification email: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit verification resend: %w", err)
	}
	return true, nil
}

func (repository *Postgres) ActiveAccountEmail(ctx context.Context, identifier string) (string, bool, error) {
	var email string
	query := `SELECT email FROM users WHERE status='active' AND login_normalized=$1`
	if strings.Contains(identifier, "@") {
		query = `SELECT email FROM users WHERE status='active' AND email_normalized=$1`
	}
	err := repository.pool.QueryRow(ctx, query, identifier).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve active account: %w", err)
	}
	return email, true, nil
}

func (repository *Postgres) CreatePasswordReset(ctx context.Context, reset account.PasswordResetIssue) (bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin password reset request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID string
	query := `SELECT id::text FROM users WHERE status='active' AND login_normalized=$1 FOR UPDATE`
	if strings.Contains(reset.IdentifierNormalized, "@") {
		query = `SELECT id::text FROM users WHERE status='active' AND email_normalized=$1 FOR UPDATE`
	}
	err = tx.QueryRow(ctx, query, reset.IdentifierNormalized).Scan(&userID)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("lock password reset account: %w", err)
	}
	if found {
		if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET revoked_at=NOW(),revoke_reason='replacement' WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, userID); err != nil {
			return false, fmt.Errorf("revoke prior password reset token: %w", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO password_reset_tokens (user_id,token_hash,hash_key_version,expires_at) VALUES ($1,$2,$3,$4)`, userID, reset.TokenHash, reset.TokenKeyVersion, reset.TokenExpiresAt); err != nil {
			return false, fmt.Errorf("insert password reset token: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version)
			VALUES ($1,'email',$2,$3,$4,$5)
		`, userID, passwordResetMessageType, reset.OutboxIdempotencyKey, reset.OutboxCiphertext, reset.OutboxKeyVersion); err != nil {
			return false, fmt.Errorf("enqueue password reset email: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id) VALUES ('auth.password_reset_requested','accepted',$1)`, reset.RequestID); err != nil {
		return false, fmt.Errorf("append password reset request audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit password reset request: %w", err)
	}
	return found, nil
}

func (repository *Postgres) CreateTelegramPasswordReset(ctx context.Context, reset account.PasswordResetIssue) (bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin Telegram password reset request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	query := `
		SELECT users.id::text
		FROM users
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		WHERE users.status='active' AND users.login_normalized=$1
		FOR UPDATE OF users,binding`
	if strings.Contains(reset.IdentifierNormalized, "@") {
		query = strings.Replace(query, "users.login_normalized=$1", "users.email_normalized=$1", 1)
	}
	var userID string
	err = tx.QueryRow(ctx, query, reset.IdentifierNormalized).Scan(&userID)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("lock Telegram password reset account: %w", err)
	}
	if found {
		if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET revoked_at=$2,revoke_reason='replacement' WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, userID, reset.TokenExpiresAt.Add(-30*time.Minute)); err != nil {
			return false, fmt.Errorf("revoke prior Telegram reset token: %w", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO password_reset_tokens (user_id,token_hash,hash_key_version,expires_at) VALUES ($1,$2,$3,$4)`, userID, reset.TokenHash, reset.TokenKeyVersion, reset.TokenExpiresAt); err != nil {
			return false, fmt.Errorf("insert Telegram reset token: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version)
			VALUES ($1,'telegram',$2,$3,$4,$5)
		`, userID, passwordResetMessageType, reset.OutboxIdempotencyKey, reset.OutboxCiphertext, reset.OutboxKeyVersion); err != nil {
			return false, fmt.Errorf("enqueue Telegram password reset: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id) VALUES ('auth.password_reset_requested','accepted',$1)`, reset.RequestID); err != nil {
		return false, fmt.Errorf("append Telegram password reset request audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit Telegram password reset request: %w", err)
	}
	return found, nil
}

func (repository *Postgres) PasswordResetContext(ctx context.Context, tokenHash []byte, keyVersion int, now time.Time, requestID string) (account.PasswordResetAccount, error) {
	var result account.PasswordResetAccount
	err := repository.pool.QueryRow(ctx, `
		SELECT users.login,users.email
		  FROM password_reset_tokens token
		  JOIN users ON users.id=token.user_id
		 WHERE token.token_hash=$1 AND token.hash_key_version=$2
		   AND token.consumed_at IS NULL AND token.revoked_at IS NULL
		   AND token.expires_at>$3 AND users.status='active'
	`, tokenHash, keyVersion, now).Scan(&result.Login, &result.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		if auditErr := repository.recordRejectedPasswordReset(ctx, requestID); auditErr != nil {
			return account.PasswordResetAccount{}, auditErr
		}
		return account.PasswordResetAccount{}, account.ErrInvalidToken
	}
	if err != nil {
		return account.PasswordResetAccount{}, fmt.Errorf("load password reset context: %w", err)
	}
	return result, nil
}

func (repository *Postgres) CompletePasswordReset(ctx context.Context, reset account.PasswordResetCompletion) error {
	parameters, err := json.Marshal(reset.PasswordParameters)
	if err != nil {
		return fmt.Errorf("encode reset password parameters: %w", err)
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var tokenID, userID string
	err = tx.QueryRow(ctx, `
		SELECT token.id::text,token.user_id::text
		  FROM password_reset_tokens token
		  JOIN users ON users.id=token.user_id
		 WHERE token.token_hash=$1 AND token.hash_key_version=$2
		   AND token.consumed_at IS NULL AND token.revoked_at IS NULL
		   AND token.expires_at>$3 AND users.status='active'
		 FOR UPDATE OF token,users
	`, reset.TokenHash, reset.TokenKeyVersion, reset.Now).Scan(&tokenID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		if auditErr := repository.recordRejectedPasswordReset(ctx, reset.RequestID); auditErr != nil {
			return auditErr
		}
		return account.ErrInvalidToken
	}
	if err != nil {
		return fmt.Errorf("lock password reset token: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET consumed_at=$2 WHERE id=$1`, tokenID, reset.Now); err != nil {
		return fmt.Errorf("consume password reset token: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE credentials SET password_hash=$2,algorithm=$3,parameters=$4,hash_version=$5,password_changed_at=$6 WHERE user_id=$1
	`, userID, reset.PasswordHash, reset.PasswordAlgorithm, parameters, reset.PasswordHashVersion, reset.Now); err != nil {
		return fmt.Errorf("replace password credential: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET security_version=security_version+1,version=version+1,updated_at=$2 WHERE id=$1`, userID, reset.Now); err != nil {
		return fmt.Errorf("advance password security version: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,$2),revoke_reason=CASE WHEN revoked_at IS NULL THEN 'password_reset' ELSE revoke_reason END WHERE user_id=$1`, userID, reset.Now); err != nil {
		return fmt.Errorf("revoke sessions after password reset: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,subject_type,subject_user_id) VALUES ('auth.password_reset_completed','success',$1,'user',$2)`, reset.RequestID, userID); err != nil {
		return fmt.Errorf("append password reset audit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset: %w", err)
	}
	return nil
}

func (repository *Postgres) recordRejectedPasswordReset(ctx context.Context, requestID string) error {
	if _, err := repository.pool.Exec(ctx, `INSERT INTO audit_events (event_type,result,reason_code,request_id) VALUES ('auth.password_reset_completed','invalid_or_expired','invalid_or_expired',$1)`, requestID); err != nil {
		return fmt.Errorf("append rejected password reset audit: %w", err)
	}
	return nil
}

func (repository *Postgres) RecordPasswordResetRequest(ctx context.Context, result, requestID string) error {
	if result != "rate_limited" {
		return fmt.Errorf("unsupported password reset request result")
	}
	if _, err := repository.pool.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id) VALUES ('auth.password_reset_requested',$1,$2)`, result, requestID); err != nil {
		return fmt.Errorf("append password reset request audit: %w", err)
	}
	return nil
}

func (repository *Postgres) RecordPasswordResetFailure(ctx context.Context, requestID string) error {
	return repository.recordRejectedPasswordReset(ctx, requestID)
}

func (repository *Postgres) ClaimEmailOutbox(ctx context.Context, now time.Time) (EmailOutboxEntry, bool, error) {
	var entry EmailOutboxEntry
	err := repository.pool.QueryRow(ctx, `
		WITH due AS (
			SELECT id
			  FROM notification_outbox
			 WHERE channel = 'email'
			   AND state IN ('pending', 'retry')
			   AND available_at <= $1
			 ORDER BY available_at, created_at, id
			 FOR UPDATE SKIP LOCKED
			 LIMIT 1
		)
		UPDATE notification_outbox outbox
		   SET state = 'delivering', attempts = attempts + 1, locked_at = $1, updated_at = $1
		  FROM due
		 WHERE outbox.id = due.id
		RETURNING outbox.id::text, outbox.message_type, outbox.payload_ciphertext,
		          outbox.payload_key_version, outbox.attempts
	`, now).Scan(&entry.ID, &entry.MessageType, &entry.PayloadCiphertext, &entry.PayloadKeyVersion, &entry.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return EmailOutboxEntry{}, false, nil
	}
	if err != nil {
		return EmailOutboxEntry{}, false, fmt.Errorf("claim email outbox: %w", err)
	}
	return entry, true, nil
}

func (repository *Postgres) RecoverStaleEmailOutbox(ctx context.Context, staleBefore, now time.Time) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
		UPDATE notification_outbox
		   SET state = 'retry', available_at = $2, locked_at = NULL,
		       last_error_code = 'worker_timeout', updated_at = $2
		 WHERE channel = 'email'
		   AND state = 'delivering'
		   AND locked_at < $1
	`, staleBefore, now)
	if err != nil {
		return 0, fmt.Errorf("recover stale email outbox: %w", err)
	}
	return result.RowsAffected(), nil
}

func (repository *Postgres) MarkEmailOutboxDelivered(ctx context.Context, id string, now time.Time) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE notification_outbox
		   SET state = 'delivered', delivered_at = $2, locked_at = NULL,
		       last_error_code = NULL, updated_at = $2
		 WHERE id = $1 AND state = 'delivering'
	`, id, now)
	if err != nil {
		return fmt.Errorf("mark email delivered: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("email outbox claim was lost")
	}
	return nil
}

func (repository *Postgres) MarkEmailOutboxFailed(ctx context.Context, id string, attempts, maximumAttempts int, now, retryAt time.Time, errorCode string) error {
	state := "retry"
	var terminalAt *time.Time
	if attempts >= maximumAttempts {
		state = "terminal"
		terminalAt = &now
	}
	result, err := repository.pool.Exec(ctx, `
		UPDATE notification_outbox
		   SET state = $2, available_at = $3, locked_at = NULL, terminal_at = $4,
		       last_error_code = $5, updated_at = $6
		 WHERE id = $1 AND state = 'delivering'
	`, id, state, retryAt, terminalAt, errorCode, now)
	if err != nil {
		return fmt.Errorf("mark email outbox failure: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("email outbox claim was lost")
	}
	return nil
}

func (repository *Postgres) RecoverStaleTelegramAccountOutbox(ctx context.Context, staleBefore, now time.Time) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
		UPDATE notification_outbox
		SET state='retry',available_at=$2,locked_at=NULL,last_error_code='worker_timeout',updated_at=$2
		WHERE channel='telegram' AND state='delivering' AND locked_at<$1
	`, staleBefore, now)
	if err != nil {
		return 0, fmt.Errorf("recover stale Telegram account outbox: %w", err)
	}
	return result.RowsAffected(), nil
}

func (repository *Postgres) TerminalizeInactiveTelegramAccountOutbox(ctx context.Context, now time.Time) (int64, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin inactive Telegram outbox cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		UPDATE notification_outbox outbox
		SET state='terminal',terminal_at=$1,locked_at=NULL,last_error_code='recipient_inactive',updated_at=$1
		WHERE outbox.channel='telegram' AND outbox.state IN ('pending','retry')
		  AND outbox.recipient_user_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM users
			JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
			WHERE users.id=outbox.recipient_user_id AND users.status='active'
		  )
		RETURNING outbox.id::text`, now)
	if err != nil {
		return 0, fmt.Errorf("terminalize inactive Telegram outbox: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE report_deliveries SET state='recipient_inactive',terminal_at=$2 WHERE outbox_id=ANY($1::uuid[]) AND state='pending'`, ids, now); err != nil {
			return 0, fmt.Errorf("terminalize inactive report delivery: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit inactive Telegram outbox cleanup: %w", err)
	}
	return int64(len(ids)), nil
}

func (repository *Postgres) ClaimTelegramAccountOutbox(ctx context.Context, now time.Time) (TelegramAccountOutboxEntry, bool, error) {
	var entry TelegramAccountOutboxEntry
	err := repository.pool.QueryRow(ctx, `
		WITH due AS (
			SELECT outbox.id,binding.chat_id
			FROM notification_outbox outbox
			JOIN users ON users.id=outbox.recipient_user_id AND users.status='active'
			JOIN telegram_account_bindings binding ON binding.user_id=outbox.recipient_user_id AND binding.revoked_at IS NULL
			WHERE outbox.channel='telegram' AND outbox.state IN ('pending','retry') AND outbox.available_at<=$1
			ORDER BY outbox.available_at,outbox.created_at,outbox.id
			FOR UPDATE OF outbox SKIP LOCKED
			LIMIT 1
		)
		UPDATE notification_outbox outbox
		SET state='delivering',attempts=attempts+1,locked_at=$1,updated_at=$1
		FROM due
		WHERE outbox.id=due.id
		RETURNING outbox.id::text,due.chat_id,outbox.message_type,outbox.payload_ciphertext,outbox.payload_key_version,outbox.attempts
	`, now).Scan(&entry.ID, &entry.ChatID, &entry.MessageType, &entry.PayloadCiphertext, &entry.PayloadKeyVersion, &entry.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return TelegramAccountOutboxEntry{}, false, nil
	}
	if err != nil {
		return TelegramAccountOutboxEntry{}, false, fmt.Errorf("claim Telegram account outbox: %w", err)
	}
	return entry, true, nil
}

func (repository *Postgres) MarkTelegramAccountOutboxDelivered(ctx context.Context, id string, now time.Time) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram outbox delivery: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `UPDATE notification_outbox SET state='delivered',delivered_at=$2,locked_at=NULL,last_error_code=NULL,updated_at=$2 WHERE id=$1 AND channel='telegram' AND state='delivering'`, id, now)
	if err != nil {
		return fmt.Errorf("mark Telegram account outbox delivered: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("Telegram account outbox claim was lost")
	}
	if _, err := tx.Exec(ctx, `UPDATE report_deliveries SET state='delivered',delivered_at=$2 WHERE outbox_id=$1 AND state='pending'`, id, now); err != nil {
		return fmt.Errorf("mark report delivery delivered: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram outbox delivery: %w", err)
	}
	return nil
}

func (repository *Postgres) MarkTelegramAccountOutboxFailed(ctx context.Context, id string, attempts, maximumAttempts int, now, retryAt time.Time, errorCode string) error {
	state := "retry"
	var terminalAt *time.Time
	if attempts >= maximumAttempts {
		state, terminalAt = "terminal", &now
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram outbox failure: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE notification_outbox
		SET state=$2,available_at=$3,locked_at=NULL,terminal_at=$4,last_error_code=$5,updated_at=$6
		WHERE id=$1 AND channel='telegram' AND state='delivering'
	`, id, state, retryAt, terminalAt, errorCode, now)
	if err != nil {
		return fmt.Errorf("mark Telegram account outbox failure: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("Telegram account outbox claim was lost")
	}
	if state == "terminal" {
		if _, err := tx.Exec(ctx, `UPDATE report_deliveries SET state='terminal',terminal_at=$2 WHERE outbox_id=$1 AND state='pending'`, id, now); err != nil {
			return fmt.Errorf("mark report delivery terminal: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram outbox failure: %w", err)
	}
	return nil
}
