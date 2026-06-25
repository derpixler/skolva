package auth_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
)

func countString(ss []string, want string) int {
	n := 0
	for _, s := range ss {
		if s == want {
			n++
		}
	}
	return n
}

func permSlugs(ps []db.Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Slug
	}
	return out
}

func TestRepositoryPermissions(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	user, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "perm@example.com", PasswordHash: "h", FirstName: "Per", LastName: "Mission",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	assignedBy := uuid.NullUUID{UUID: user.ID, Valid: true}

	// all seeded permissions
	all, err := repo.ListPermissions(ctx)
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	if len(all) != 47 {
		t.Errorf("expected 47 seeded permissions, got %d", len(all))
	}

	// no roles -> no permissions
	if perms, err := repo.GetPermissionsForUser(ctx, user.ID); err != nil || len(perms) != 0 {
		t.Fatalf("expected no permissions, got %+v err=%v", perms, err)
	}

	// mitglied -> 7 read permissions
	if err := repo.AssignRole(ctx, user.ID, "mitglied", assignedBy); err != nil {
		t.Fatalf("assign mitglied: %v", err)
	}
	perms, err := repo.GetPermissionsForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("perms for user: %v", err)
	}
	if len(perms) != 7 {
		t.Errorf("expected 7 permissions for mitglied, got %d (%v)", len(perms), perms)
	}
	if countString(perms, "units.read") != 1 {
		t.Errorf("expected units.read once, got %v", perms)
	}
	if countString(perms, "users.write") != 0 {
		t.Errorf("mitglied must not have users.write, got %v", perms)
	}

	// + kassierer -> union, DISTINCT on overlapping units.read
	if err := repo.AssignRole(ctx, user.ID, "kassierer", assignedBy); err != nil {
		t.Fatalf("assign kassierer: %v", err)
	}
	perms, err = repo.GetPermissionsForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("perms for user (2 roles): %v", err)
	}
	if countString(perms, "units.read") != 1 {
		t.Errorf("expected units.read exactly once (DISTINCT), got %d", countString(perms, "units.read"))
	}
	if countString(perms, "accounting.write") != 1 {
		t.Errorf("expected accounting.write (kassierer), got %v", perms)
	}
	if countString(perms, "lending.read") != 1 {
		t.Errorf("expected lending.read (mitglied), got %v", perms)
	}

	// role_permissions read + write
	rolePerms, err := repo.GetPermissionsForRole(ctx, "mitglied")
	if err != nil {
		t.Fatalf("perms for role: %v", err)
	}
	if len(rolePerms) != 7 {
		t.Errorf("expected 7 permissions for role mitglied, got %d", len(rolePerms))
	}
	if err := repo.AddRolePermission(ctx, "mitglied", "users.read"); err != nil {
		t.Fatalf("add role permission: %v", err)
	}
	rolePerms, _ = repo.GetPermissionsForRole(ctx, "mitglied")
	if len(rolePerms) != 8 || countString(permSlugs(rolePerms), "users.read") != 1 {
		t.Errorf("expected 8 incl users.read, got %v", permSlugs(rolePerms))
	}
	if err := repo.RemoveRolePermission(ctx, "mitglied", "users.read"); err != nil {
		t.Fatalf("remove role permission: %v", err)
	}
	rolePerms, _ = repo.GetPermissionsForRole(ctx, "mitglied")
	if len(rolePerms) != 7 {
		t.Errorf("expected 7 after remove, got %d", len(rolePerms))
	}

	// admin resolves to all 47 permissions
	admin, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "admin@example.com", PasswordHash: "h", FirstName: "Ad", LastName: "Min",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := repo.AssignRole(ctx, admin.ID, "admin", uuid.NullUUID{UUID: admin.ID, Valid: true}); err != nil {
		t.Fatalf("assign admin: %v", err)
	}
	adminPerms, err := repo.GetPermissionsForUser(ctx, admin.ID)
	if err != nil {
		t.Fatalf("admin perms: %v", err)
	}
	if len(adminPerms) != 47 {
		t.Errorf("expected admin to have all 47 permissions, got %d", len(adminPerms))
	}
}
