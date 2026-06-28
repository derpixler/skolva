package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestRepositoryRolesAndUserRoles(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	user, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "member@example.com", PasswordHash: "h", FirstName: "Mem", LastName: "Ber",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	assignedBy := uuid.NullUUID{UUID: user.ID, Valid: true}

	// seeded roles
	roles, err := repo.ListRoles(ctx)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(roles) != 5 {
		t.Errorf("expected 5 seeded roles, got %d", len(roles))
	}
	role, err := repo.GetRole(ctx, "admin")
	if err != nil || role.DisplayName != "Administrator" {
		t.Errorf("get role admin: row=%+v err=%v", role, err)
	}
	if _, err := repo.GetRole(ctx, "does-not-exist"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected ErrNoRows for unknown role, got %v", err)
	}

	// assign (idempotent)
	if err := repo.AssignRole(ctx, user.ID, "mitglied", assignedBy); err != nil {
		t.Fatalf("assign mitglied: %v", err)
	}
	if err := repo.AssignRole(ctx, user.ID, "mitglied", assignedBy); err != nil {
		t.Fatalf("re-assign mitglied (should be idempotent): %v", err)
	}
	got, err := repo.ListUserRoles(ctx, user.ID)
	if err != nil || len(got) != 1 || got[0].Slug != "mitglied" {
		t.Fatalf("expected [mitglied], got %+v err=%v", got, err)
	}

	if ok, err := repo.UserHasRole(ctx, user.ID, "mitglied"); err != nil || !ok {
		t.Errorf("expected user to have mitglied, got ok=%v err=%v", ok, err)
	}
	if ok, err := repo.UserHasRole(ctx, user.ID, "admin"); err != nil || ok {
		t.Errorf("expected user not to have admin, got ok=%v err=%v", ok, err)
	}

	// add admin -> sorted by slug: admin, mitglied
	if err := repo.AssignRole(ctx, user.ID, "admin", assignedBy); err != nil {
		t.Fatalf("assign admin: %v", err)
	}
	got, err = repo.ListUserRoles(ctx, user.ID)
	if err != nil || len(got) != 2 || got[0].Slug != "admin" || got[1].Slug != "mitglied" {
		t.Fatalf("expected [admin, mitglied], got %+v err=%v", got, err)
	}

	// remove mitglied
	if err := repo.RemoveRole(ctx, user.ID, "mitglied"); err != nil {
		t.Fatalf("remove mitglied: %v", err)
	}
	got, err = repo.ListUserRoles(ctx, user.ID)
	if err != nil || len(got) != 1 || got[0].Slug != "admin" {
		t.Fatalf("expected [admin] after remove, got %+v err=%v", got, err)
	}

	// assigning a non-existent role violates the FK
	if err := repo.AssignRole(ctx, user.ID, "no-such-role", assignedBy); err == nil {
		t.Error("expected FK error assigning a non-existent role")
	}
}
