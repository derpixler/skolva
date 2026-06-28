#!/usr/bin/env bash
# =============================================================================
# TP2 Test Steps — Identity + CRM + Groups
# =============================================================================

# --- Unit Tests (no Docker required) ---

tp2_unit() {
    banner "TP2 Unit Tests"

    step "1.1" "Auth — password hashing (bcrypt)"
    run_go_test "password" ./internal/auth/ -v -run "TestHash"

    step "1.2" "Auth — JWT token manager"
    run_go_test "JWT" ./internal/auth/ -v -run "TestNewTokenManager|TestIssue|TestVerify"

    step "1.3" "Middleware — CORS, RequestID, Authenticate, Permissions"
    run_go_test "middleware" ./internal/core/middleware/ -v

    step "1.4" "Mail — NoopMailer, SMTP, HTML templates"
    run_go_test "mail" ./internal/core/mail/ -v

    step "1.5" "Secrets — AES-256-GCM cipher"
    run_go_test "secrets" ./internal/core/secrets/ -v

    step "1.6" "Metadata — allow-list unit"
    run_go_test "metadata unit" ./internal/core/metadata/ -v -run "TestNewStore"

    step "1.7" "Search — allow-list + empty-query unit"
    run_go_test "search unit" ./internal/core/search/ -v -run "TestNewSearcher|TestSearchEmpty"

    step "1.8" "Auth — CheckPermission service (admin bypass)"
    run_go_test "CheckPermission" ./internal/auth/ -v -run "TestServiceCheckPermission"

    step "1.9" "Config — ENCRYPTION_KEY + defaults"
    run_go_test "config" ./internal/core/config/ -v

    step "1.10" "Types — Decimal + Duration"
    run_go_test "types" ./internal/core/types/ -v

    step "1.11" "Errors — PG Error Mapping"
    run_go_test "errors" ./internal/core/errors/ -v

    step "1.12" "OpenAPI — spec validation + route parity"
    run_go_test "openapi" ./internal/app/ -v -run "TestOpenAPI"
}

# --- Integration Tests (needs Docker / testcontainers) ---

tp2_integration() {
    banner "TP2 Integration Tests (testcontainers)"

    if ! docker_available; then
        echo -e "  ${YELLOW}[SKIP]${NC} Docker not available — skipping TP2 integration tests"
        return
    fi

    step "2.1" "dbexec — Actor transaction wrapper (audit + soft-delete)"
    run_go_test "dbexec" ./internal/core/dbexec/ -v -count=1

    step "2.2" "Auth Repository — users (CRUD, soft-delete, audit)"
    run_go_test "auth repo users" ./internal/auth/ -v -count=1 -run "TestRepositoryUser"

    step "2.3" "Auth Repository — roles + user_roles (assign/remove/list)"
    run_go_test "auth repo roles" ./internal/auth/ -v -count=1 -run "TestRepositoryRoles"

    step "2.4" "Auth Repository — permissions + resolution"
    run_go_test "auth repo perms" ./internal/auth/ -v -count=1 -run "TestRepositoryPermissions"

    step "2.5" "Auth HTTP — role assignment endpoints (#17)"
    run_go_test "auth handler roles" ./internal/auth/ -v -count=1 -run "TestRoleAssignment"

    step "2.6" "Metadata — EAV CRUD (get/set/delete/all)"
    run_go_test "metadata CRUD" ./internal/core/metadata/ -v -count=1 -run "TestStoreCRUD"

    step "2.7" "Search — full-text users (German stemming)"
    run_go_test "search CRUD" ./internal/core/search/ -v -count=1 -run "TestSearchUsers"

    step "2.8" "Auth HTTP — Login (valid/invalid/claims)"
    run_go_test "login" ./internal/auth/ -v -count=1 -run "TestLoginEndpoint"

    step "2.9" "Auth HTTP — Register + Users CRUD (#128)"
    run_go_test "users CRUD" ./internal/auth/ -v -count=1 -run "TestUserEndpoints"

    step "2.10" "Auth HTTP — 2FA flow (setup/confirm/verify/recovery/disable)"
    run_go_test "2FA" ./internal/auth/ -v -count=1 -run "Test2FAFlow"

    step "2.11" "Auth E2E — register → login → 2FA → profile (#25)"
    run_go_test "E2E" ./internal/auth/ -v -count=1 -run "TestE2ERegister"

    step "2.12" "CRM — address (upsert/get/1:1)"
    run_go_test "crm address" ./internal/crm/ -v -count=1 -run "TestAddress"

    step "2.13" "CRM — preferences (upsert/get)"
    run_go_test "crm preferences" ./internal/crm/ -v -count=1 -run "TestPreferences"

    step "2.14" "CRM — contacts (CRUD, is_primary, audit, soft-delete)"
    run_go_test "crm contacts" ./internal/crm/ -v -count=1 -run "TestContacts"

    step "2.15" "CRM — address + preferences endpoints"
    run_go_test "crm handler" ./internal/crm/ -v -count=1 -run "TestAddressAndPreferences"

    step "2.16" "CRM — contacts endpoints"
    run_go_test "crm contacts HTTP" ./internal/crm/ -v -count=1 -run "TestContactEndpoints"

    step "2.17" "Groups — CRUD (create/get/list/find-by-type/update/soft-delete, audit)"
    run_go_test "groups repo" ./internal/groups/ -v -count=1 -run "TestGroupRepository"

    step "2.18" "Groups — endpoints (CRUD + filter)"
    run_go_test "groups handler" ./internal/groups/ -v -count=1 -run "TestGroupEndpoints"

    step "2.19" "Group Members — repository (add/remove/list, change-role)"
    run_go_test "members repo" ./internal/groups/ -v -count=1 -run "TestGroupMembersRepository"

    step "2.20" "Group Members — endpoints (add/remove/list/user-groups)"
    run_go_test "members handler" ./internal/groups/ -v -count=1 -run "TestGroupMemberEndpoints"

    step "2.21" "Auth HTTP — Email-2FA (setup/confirm/verify/resend/disable + lockout) (#134)"
    run_go_test "Email-2FA" ./internal/auth/ -v -count=1 -run "TestEmail2FAFlow|TestEmail2FALockout"
}
