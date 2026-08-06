# rustore-fdroid

Go CLI tool that bridges RuStore apps into F-Droid repositories. It fetches app metadata and APKs, maintains `index-v1.json`, and publishes `index-v1.jar` plus index v2 metadata authenticated by a signed `entry.jar`.

## Build & Run

```bash
go build -o rustore-fdroid .
./rustore-fdroid -r ./repo <command>
```

## Project Structure

- `main.go` — entry point
- `cmd/` — Cobra commands (`init`, `add`, `remove`, `list`, `update`, `sign`, and `frontend add|remove`)
- `internal/` — core logic:
  - `rustore.go`, `http.go`, `retry.go` — RuStore API access, scoped CA trust, and retries
  - `download.go` — atomic downloads, hashing, and progress reporting
  - `apkmeta.go`, `apksig.go` — APK metadata, permission, and signer extraction
  - `index.go`, `indexv2.go` — F-Droid index v1/v2 types and builders
  - `jarsign.go` — JAR/PKCS #7 signing and repository publication
  - `removals.go` — deferred file removal after successful signing
- `web/` — embedded optional frontend, package pages, signing information, and QR generation

## Key Dependencies

- `spf13/cobra` — CLI framework
- `go.mozilla.org/pkcs7` — repository signing and APK v1 certificate parsing
- `cespare/xxhash/v2` — existing APK verification against RuStore hashes
- `schollz/progressbar/v3` — download progress
- `shogo82148/androidbinary` — APK manifest metadata and permission parsing
- `yeqown/go-qrcode/v2` — frontend QR generation

## Conventions

- Every runnable repository subcommand requires the persistent `-r`/`--repo` flag
- `init` creates `index-v1.json`; app operations add APK and media files
- `sign` publishes `index-v1.jar`, `index-v2.json`, `entry.json`, and `entry.jar`
- Superseded files and files queued by `remove` are retained until the next successful `sign`
- RuStore API base: `https://backapi.rustore.ru/applicationData`
- Extra CA trust is restricted to `rustore.ru` and its subdomains; TLS verification is never disabled
- Tests exist under `internal/`, `cmd/`, and `web/`; run `go test ./...`

## Commit Style

Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```text
<type>(<scope>): <description>
```

- **Types**: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `build`
- **Scopes**: `cli`, `api`, `index`, `sign`, `download`, `web`
- **Scope is required** — always include it
- Breaking changes: add `!` after the scope (e.g. `feat(cli)!: ...`)
- Keep the subject line under 72 characters

## Versioning

The project uses [Semantic Versioning](https://semver.org/) via git tags (e.g. `v0.2.1`).

- **patch** (for example, `v0.1.1`): backward-compatible bug fixes
- **minor** (for example, `v0.2.0`): backward-compatible features
- **major** (for example, `v1.0.0`): breaking changes
- After committing a `feat` or breaking change, create a new version tag accordingly
- **Never force-move tags** — Go module proxy caches tag contents permanently; a moved tag will serve stale code to `go install` users. If a tag was released with a bug, bump the version instead
- Version is set via `cmd.Version` variable: `go install` picks it up from `debug.ReadBuildInfo()`, local builds can use `-ldflags "-X github.com/visionavtr/rustore-fdroid/cmd.Version=vX.Y.Z"`
