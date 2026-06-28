# Passwort-Reset

## Ablauf im Detail

### Schritt 1: `POST /api/auth/password/forgot`

```
POST /api/auth/password/forgot  {"email":"user@example.com"}
← 200 {"message":"If the email exists, a reset link has been sent."}
```

**Server (service.ForgotPassword):**
1. `repo.GetUserByEmail(email)` — User suchen
2. User **nicht** gefunden → trotzdem **200 OK** (keine User-Enumeration)
3. User gefunden:
   - Token generieren: `uuid.NewString()` (kryptografisch zufällig)
   - `bcrypt.GenerateFromPassword([]byte(token), cost)` → `tokenHash`
   - In `users_meta` (EAV, #23) speichern:
     - `auth.reset.token_hash` = `tokenHash`
     - `auth.reset.expires_at` = `time.Now().Add(1h).UTC().Format(time.RFC3339)`
   - Mail via `core/mail.Mailer` (#22) senden mit Link:
     ```
     https://skolva.example.com/reset?user=<userID>&token=<token>
     Der Link ist 1 Stunde gültig.
     ```

### Schritt 2: `POST /api/auth/password/reset`

```
POST /api/auth/password/reset  {"user_id":"<uuid>","token":"<uuid>","password":"neuesPasswort"}
← 204
```

**Server (service.ResetPassword):**
1. `repo.GetUserByID(ctx, userID)` → User existent? Nicht → 404
2. `metadata.Store("users_meta").Get("auth.reset.used")` → `"true"` → 422 „bereits verwendet"
3. `metadata.Store("users_meta").Get("auth.reset.expires_at")` → abgelaufen → 422 „abgelaufen"
4. `metadata.Store("users_meta").Get("auth.reset.token_hash")` → fehlt → 422 „kein aktiver Reset"
5. `bcrypt.CompareHashAndPassword(tokenHash, []byte(token))` → stimmt nicht → 422 „ungültig"
6. `HashPassword(newPassword)` → neuer Hash
7. `repo.UpdatePassword(ctx, userID, newHash)` — Passwort in DB aktualisieren
8. `metadata.Store("users_meta").Set("auth.reset.used", "true")` — Token verbrauchen
9. **204 No Content**

## Security-Eigenschaften

| Eigenschaft | Umsetzung |
|---|---|
| **Keine User-Enumeration** | `POST /forgot` antwortet immer 200, auch bei unbekannter Email |
| **Token-Expiry** | 1 Stunde, via `auth.reset.expires_at` in `users_meta` |
| **Single-Use** | Nach erfolgreichem Reset wird `auth.reset.used = "true"` gesetzt |
| **Token-Hashing** | Reset-Token wird bcrypt-gehasht in `users_meta` gespeichert |
| **Passwort-Hashing** | Neues Passwort via `bcrypt.GenerateFromPassword` (#1a) |

## Beteiligte Komponenten

| Komponente | Zweck |
|---|---|
| `internal/auth/password_reset.go` | Service: `ForgotPassword`, `ResetPassword` |
| `internal/auth/repository.go` | `UpdatePassword` (sqlc-Query) |
| `internal/core/metadata` (#23) | EAV-Store für Token-Hash, Expiry, Used-Flag in `users_meta` |
| `internal/core/mail` (#22) | Mail-Versand des Reset-Links |
| `sqlc/queries/users.sql` | `UpdatePassword`-Query |

## Endpunkte

| Endpoint | Auth | Beschreibung |
|---|---|---|
| `POST /api/auth/password/forgot` | public | Reset-Link anfordern |
| `POST /api/auth/password/reset` | public | Passwort mit Token zurücksetzen |

## Meta-Keys in `users_meta`

| Key | Typ | Beschreibung |
|---|---|---|
| `auth.reset.token_hash` | string | bcrypt-Hash des Reset-Tokens |
| `auth.reset.expires_at` | string (RFC3339) | Ablaufzeitpunkt |
| `auth.reset.used` | string (`"true"`) | Wird nach erfolgreichem Reset gesetzt |
