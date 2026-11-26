//go:build integration

package repository

import (
	"context"
	"slices"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestReleaseNoteReadsArePerAccountAndConcurrentIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "release_notes")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	reader := insertProjectTestUser(t, pool, "release.reader", "reader@example.com")
	other := insertProjectTestUser(t, pool, "release.other", "other@example.com")
	ctx := context.Background()
	group, child := errgroup.WithContext(ctx)
	for range 3 {
		group.Go(func() error { return store.MarkReleaseVersionRead(child, reader, "1.0.19") })
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	read, err := store.ReadReleaseVersions(ctx, reader, []string{"1.0.19", "1.0.20"})
	if err != nil || !slices.Equal(read, []string{"1.0.19"}) {
		t.Fatalf("reader state: %v, %v", read, err)
	}
	read, err = store.ReadReleaseVersions(ctx, other, []string{"1.0.19"})
	if err != nil || len(read) != 0 {
		t.Fatalf("other account saw read state: %v, %v", read, err)
	}
	if err = store.MarkReleaseVersionRead(ctx, reader, "invalid"); err == nil {
		t.Fatal("expected invalid version to be rejected")
	}
	if _, err = store.ReadReleaseVersions(ctx, "invalid-uuid", []string{"1.0.19"}); err == nil {
		t.Fatal("expected invalid account ID to be rejected")
	}
}
