# tools — users: create first user & dump logins

Helper CLIs for the `users` table in `canyon-springs.db` (SQLite).

| Tool | What it does | Source |
|------|--------------|--------|
| `create-admin` | Creates one user in the DB (first user on a fresh DB, or any later user). Passwords are bcrypt-hashed via `store.CreateUser`. | `tools/create-admin/main.go` |
| `dump-logins` | Dumps all logins as `username:bcrypt_hash` lines. Used to verify who exists on a DB. | `tools/dump-logins/main.go` |
| `create-user/` | Empty placeholder directory, currently unused. Use `create-admin` for all user creation. | — |

Both tools call `store.Open(path)` which runs `store.Initialize(db)`, so **you do not need to pre-create the database** — pointing at a non-existent `.db` path creates a fresh DB with the full schema (`suites`, `residents`, `users`, `sessions`, etc.).

Run all commands from the **repo root** (`c:\dev\csDB`, where `go.mod` lives).

## 1. Put a user in a fresh database

On a fresh/empty database there are 0 rows in `users`, so `/login` will reject every login with `No users configured. Use tools/create-admin to create one.` You must seed at least one user (ideally an admin) with `create-admin`.

Flags for `create-admin` (see `tools/create-admin/main.go`):

```
-db     path to database (default "canyon-springs.db", relative to where you run it)
-user   username (required)
-pass   password (required)
-admin  grant admin role (default false)
```

It exits non-zero with `user already exists: <name>` if the username is taken.

### Example: first admin on a brand-new DB (PowerShell)

```powershell
cd c:\dev\csDB
# creates canyon-springs.db in the current dir if it does not exist
go run ./tools/create-admin -user admin -pass "ChangeMe123!" -admin
```

### Example: first admin on a brand-new DB (cmd / bash)

```sh
cd c:/dev/csDB
go run ./tools/create-admin -user admin -pass "ChangeMe123!" -admin
```

### Example: custom DB path

```powershell
go run ./tools/create-admin -db C:\data\canyon-springs.db -user admin -pass "ChangeMe123!" -admin
```

Make sure the server uses the **same file**. The server resolves the DB as:

```
$env:CANYON_DB (if set) else ./canyon-springs.db
```

So either set `$env:CANYON_DB="C:\data\canyon-springs.db"` before starting `server.exe` / `go run .`, or keep the default path.

### Add more users later

```powershell
# regular (non-admin) user
go run ./tools/create-admin -user alice -pass "s3cret!" 

# second admin
go run ./tools/create-admin -user bob -pass "s3cret!" -admin

# explicit db
go run ./tools/create-admin -db canyon-springs.db -user alice -pass "s3cret!"
```

Once you can log in as an admin you can also manage users in the web UI at `http://localhost:8081/admin` (create user, toggle admin, reset password, delete) — no need for the CLI after bootstrap.

## 2. Dump the users

`dump-logins` takes an optional positional arg: the DB path. Default is `canyon-springs.db` in the current directory.

```powershell
cd c:\dev\csDB

# dump default DB
go run ./tools/dump-logins

# dump explicit DB
go run ./tools/dump-logins canyon-springs.db
go run ./tools/dump-logins C:\data\canyon-springs.db
```

Output is one line per user, sorted by username:

```
admin:$2a$10$...
alice:$2a$10$...
```

`no users` is printed when the `users` table is empty (i.e. fresh DB, no one created yet).

> Security note: this prints **bcrypt password hashes, not plaintext passwords**. There is no tool to recover a plaintext password — if a user forgot it, reset it: either log in as admin and use `/admin` → password reset (`store.UpdatePassword`), or delete + re-create the user with `create-admin`.

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `user already exists: alice` | Pick another name, or delete/reset via `/admin` UI. |
| `flag.Usage()` / exit 1 with no message | You omitted `-user` or `-pass`. Both are required. |
| Server still says `No users configured` | You seeded a different file than the server opened. Check `$env:CANYON_DB` and your `-db` / positional arg; use absolute paths to be safe. |
| `no users` from `dump-logins` | Expected on a fresh DB. Create one with `create-admin` (section 1). |
| Login `Invalid username or password` | Username is case-sensitive (`store.FindUser` does exact match); verify with `dump-logins` that the name exists, then reset the password via `/admin` or re-create. |

## Build once (optional)

```powershell
go build -o create-admin.exe ./tools/create-admin
go build -o dump-logins.exe ./tools/dump-logins
.\create-admin.exe -user admin -pass "ChangeMe123!" -admin
.\dump-logins.exe canyon-springs.db
```