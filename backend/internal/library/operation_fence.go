package library

import (
	"context"
	"database/sql"
)

// Keep the ownership row locked across the short filesystem mutation. Checking a
// lease before unlink/link alone leaves a takeover window between check and IO.
// Hashing/transfer is normally outside this section; replacement checks may read
// existing bytes here before retiring their name. No remote client calls occur.
func (s *Store) withOperationFence(ctx context.Context, op ImportOperation, action func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var valid bool
	if err := tx.QueryRowContext(ctx, `select coalesce(lease_token=$2::uuid and lease_expires_at>clock_timestamp(),false) from import_operations where id=$1 for update`, op.ID, op.LeaseToken).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return ErrImportBusy
	}
	if err := action(tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update import_operations set lease_expires_at=clock_timestamp()+interval '2 minutes' where id=$1 and lease_token=$2`, op.ID, op.LeaseToken); err != nil {
		return err
	}
	return tx.Commit()
}
