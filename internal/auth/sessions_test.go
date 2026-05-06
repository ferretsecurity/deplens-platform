package auth_test

import (
	"context"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

func TestSessionManagerCanCommitMembershipRecords(t *testing.T) {
	manager := auth.NewSessionManager(auth.SessionConfig{})

	ctx, err := manager.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	manager.Put(ctx, "memberships", []store.MembershipRecord{
		{
			TenantID:   "tenant-123",
			TenantSlug: "default",
			Role:       "owner",
		},
	})

	if _, _, err := manager.Commit(ctx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}
