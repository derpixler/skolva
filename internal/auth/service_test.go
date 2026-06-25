package auth_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
)

func TestServiceCheckPermission(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)
	svc := auth.NewService(repo, nil)

	// seed sanity: 47 permissions across 5 roles
	perms, err := repo.ListPermissions(ctx)
	if err != nil || len(perms) != 47 {
		t.Fatalf("expected 47 seeded permissions, got %d err=%v", len(perms), err)
	}
	roles, err := repo.ListRoles(ctx)
	if err != nil || len(roles) != 5 {
		t.Fatalf("expected 5 seeded roles, got %d err=%v", len(roles), err)
	}

	mkUser := func(email string) uuid.UUID {
		t.Helper()
		u, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
			Email: email, PasswordHash: "h", FirstName: "F", LastName: "L",
		})
		if err != nil {
			t.Fatalf("create %s: %v", email, err)
		}
		return u.ID
	}

	// kassierer: has accounting.write via role_permissions, not users.write
	kass := mkUser("kass@example.com")
	if err := repo.AssignRole(ctx, kass, "kassierer", uuid.NullUUID{}); err != nil {
		t.Fatalf("assign kassierer: %v", err)
	}
	if ok, err := svc.CheckPermission(ctx, kass, "accounting.write"); err != nil || !ok {
		t.Errorf("kassierer should have accounting.write: ok=%v err=%v", ok, err)
	}
	if ok, err := svc.CheckPermission(ctx, kass, "users.write"); err != nil || ok {
		t.Errorf("kassierer must not have users.write: ok=%v err=%v", ok, err)
	}

	// admin bypass: every permission implicitly, even one not in the catalog
	adm := mkUser("adm@example.com")
	if err := repo.AssignRole(ctx, adm, "admin", uuid.NullUUID{}); err != nil {
		t.Fatalf("assign admin: %v", err)
	}
	if ok, err := svc.CheckPermission(ctx, adm, "users.write"); err != nil || !ok {
		t.Errorf("admin should pass users.write: ok=%v err=%v", ok, err)
	}
	if ok, err := svc.CheckPermission(ctx, adm, "totally.made.up"); err != nil || !ok {
		t.Errorf("admin bypass should grant any permission: ok=%v err=%v", ok, err)
	}

	// user without roles has no permissions
	none := mkUser("none@example.com")
	if ok, err := svc.CheckPermission(ctx, none, "users.read"); err != nil || ok {
		t.Errorf("user without roles should have no permission: ok=%v err=%v", ok, err)
	}
}
