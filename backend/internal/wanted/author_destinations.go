package wanted

import (
	"context"
	"database/sql"
	"errors"
)

// Keep format validation and the subscription write under the same root lock.
// A later root-format change is checked again when a wanted book is created.
func validateAuthorRoot(ctx context.Context, tx *sql.Tx, id, format string) error {
	if id == "" {
		return nil
	}
	var rootFormat string
	err := tx.QueryRowContext(ctx, `select media_format from root_folders where id::text=$1 for share`, id).Scan(&rootFormat)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("root folder not found")
	}
	if err != nil {
		return err
	}
	if reason := rootFolderFormatMismatchReason(rootFormat, format); reason != "" {
		return errors.New(reason)
	}
	return nil
}
