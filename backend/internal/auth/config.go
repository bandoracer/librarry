package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SaveConfig serializes single-user configuration changes across processes and
// commits the method, credential change and session revocation in one transaction.
func (s *Store) SaveConfig(ctx context.Context, method, username, passwordHash string) error {
	if !s.Configured() {
		return ErrUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(761906101)`); err != nil {
		return err
	}
	existing, err := scanUser(tx.QueryRowContext(ctx, `select id::text, username, password_hash, created_at, updated_at from users order by created_at limit 1 for update`))
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if username != "" {
		if passwordHash == "" {
			if !exists {
				return fmt.Errorf("%w: password is required to create the user", ErrInvalidConfig)
			}
			passwordHash = existing.PasswordHash
		}
		if exists {
			if _, err := tx.ExecContext(ctx, `update users set username=$2, password_hash=$3, updated_at=now() where id=$1`, existing.ID, username, passwordHash); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `insert into users(username,password_hash) values($1,$2)`, username, passwordHash); err != nil {
				return err
			}
		}
		if !exists || username != existing.Username || passwordHash != existing.PasswordHash {
			if _, err := tx.ExecContext(ctx, `delete from sessions`); err != nil {
				return err
			}
		}
	} else if method != MethodNone && !exists {
		return fmt.Errorf("%w: username and password are required to enable authentication", ErrInvalidConfig)
	}
	if _, err := tx.ExecContext(ctx, `insert into compat_resources(resource_type,compat_id,name,payload)
        values('auth-config',1,'auth-config',jsonb_build_object('method',$1::text))
        on conflict(resource_type,compat_id) do update set payload=excluded.payload, deleted_at=null, updated_at=now()`, method); err != nil {
		return err
	}
	return tx.Commit()
}
