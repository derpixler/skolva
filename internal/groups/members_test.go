package groups_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/db"
	"github.com/derpixler/skolva/internal/groups"
	"github.com/google/uuid"
)

func TestGroupMembersRepository(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := groups.NewRepository(pool)

	var actorID, memberID uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "actor@example.com", "h").Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}
	if err := pool.QueryRow(ctx, insertUser, "member@example.com", "h").Scan(&memberID); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	actor := uuid.NullUUID{UUID: actorID, Valid: true}

	g, err := repo.Create(ctx, actorID, db.CreateGroupParams{Name: "Team A", GroupType: "mannschaft", Actor: actor})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// add member
	if err := repo.AddMember(ctx, db.AddMemberParams{
		GroupID: g.ID, UserID: memberID, RoleInGroup: "trainer", CreatedBy: actor,
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	members, err := repo.ListMembers(ctx, g.ID)
	if err != nil || len(members) != 1 || members[0].UserID != memberID || members[0].RoleInGroup != "trainer" {
		t.Fatalf("list members: rows=%+v err=%v", members, err)
	}

	// upsert -> change role
	if err := repo.AddMember(ctx, db.AddMemberParams{
		GroupID: g.ID, UserID: memberID, RoleInGroup: "leiter", CreatedBy: actor,
	}); err != nil {
		t.Fatalf("re-add member: %v", err)
	}
	members, _ = repo.ListMembers(ctx, g.ID)
	if len(members) != 1 || members[0].RoleInGroup != "leiter" {
		t.Errorf("expected single member with role leiter, got %+v", members)
	}

	// user groups
	ug, err := repo.ListUserGroups(ctx, memberID)
	if err != nil || len(ug) != 1 || ug[0].ID != g.ID || ug[0].RoleInGroup != "leiter" {
		t.Fatalf("list user groups: rows=%+v err=%v", ug, err)
	}

	// remove
	if err := repo.RemoveMember(ctx, g.ID, memberID); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if members, _ := repo.ListMembers(ctx, g.ID); len(members) != 0 {
		t.Errorf("expected no members after remove, got %d", len(members))
	}
	if ug, _ := repo.ListUserGroups(ctx, memberID); len(ug) != 0 {
		t.Errorf("expected no user groups after remove, got %d", len(ug))
	}
}
