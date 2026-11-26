package repository

import (
	"context"
	"fmt"
)

func (repository *Postgres) ReadReleaseVersions(ctx context.Context, userID string, versions []string) ([]string, error) {
	var read []string
	err := repository.pool.QueryRow(ctx, `
		SELECT COALESCE(array_agg(version), ARRAY[]::text[])
		FROM release_note_reads WHERE user_id=$1 AND version=ANY($2::text[])
	`, userID, versions).Scan(&read)
	if err != nil {
		return nil, fmt.Errorf("list read release versions: %w", err)
	}
	return read, nil
}

func (repository *Postgres) MarkReleaseVersionRead(ctx context.Context, userID, version string) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO release_note_reads (user_id, version) VALUES ($1,$2)
		ON CONFLICT (user_id, version) DO NOTHING
	`, userID, version)
	if err != nil {
		return fmt.Errorf("mark release version read: %w", err)
	}
	return nil
}
