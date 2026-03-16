# rustore-fdroid

Go CLI tool that bridges RuStore apps into F-Droid repositories. Fetches app metadata and APKs from the RuStore API, generates an F-Droid `index-v1.json`, and signs it as a JAR.

## Build & Run

```bash
go build -o rustore-fdroid .
./rustore-fdroid -r ./repo <command>
```

## Project Structure

- `main.go` — entry point
- `cmd/` — Cobra command definitions (add, remove, list, update, init, sign)
- `internal/` — core logic:
  - `rustore.go` — RuStore API client (app info, download links)
  - `index.go` — F-Droid index-v1 types, load/save, helpers
  - `jarsign.go` — JAR/PKCS7 signing for the F-Droid repo
  - `download.go` — file downloader with progress bar

## Key Dependencies

- `spf13/cobra` — CLI framework
- `go.mozilla.org/pkcs7` — PKCS7 signing
- `cespare/xxhash/v2` — fast hashing for APK dedup
- `schollz/progressbar/v3` — download progress

## Conventions

- All commands require `-r`/`--repo` flag for the repository path
- The repo directory contains: `index-v1.json`, `icons/`, APK files, and `index-v1.jar` (after signing)
- RuStore API base: `https://backapi.rustore.ru/applicationData`
- No tests yet — use the `test-writer` agent to generate them

## Commit Style

Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<scope>): <description>
```

- **Types**: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `build`
- **Scopes**: `cli`, `api`, `index`, `sign`, `download`, `web`
- **Scope is required** — always include it
- Breaking changes: add `!` after type/scope (e.g. `feat!: ...`)
- Keep the subject line under 72 characters
