# rustore-fdroid

CLI tool to generate and manage [F-Droid](https://f-droid.org/) repositories populated with apps from [RuStore](https://rustore.ru/). The `sign` command publishes both index v1 and index v2 metadata. An optional read-only web frontend lets users browse apps and add the repository to an F-Droid client.

## Install

```bash
go install github.com/visionavtr/rustore-fdroid@latest
```

Or build from source:

```bash
git clone https://github.com/visionavtr/rustore-fdroid.git
cd rustore-fdroid
go build -o rustore-fdroid .
```

## Usage

Every repository operation requires the `-r`/`--repo` flag pointing to the repository directory.

### Initialize a new repository

```bash
rustore-fdroid -r ./repo init -n "My Repo" -d "My F-Droid repository" -a "https://example.com/repo" --frontend
```

Use `--frontend` to include the web UI in the repo directory.

### Add apps

```bash
rustore-fdroid -r ./repo add <package_id> [package_id...]
```

Fetches app metadata, current release notes, and developer contacts from RuStore. It also downloads APKs, icons, and screenshots, and extracts compatibility metadata from each APK. Multiple package IDs can be processed in one call, with metadata fetched in parallel. If an APK is already present and its xxHash matches RuStore metadata, the download is skipped.

When a new APK version is added, it replaces older versions in the working index. Superseded files remain on disk until the next successful `sign`, which prevents a failed signing operation from breaking the previously published repository.

TLS verification remains enabled for downloads. The official Russian Trusted Root CA bundled with the binary is applied only to `rustore.ru` and its subdomains.

### Update apps

```bash
rustore-fdroid -r ./repo update [package_id...]
```

Updates the specified apps, or every app in the repository when no package IDs are given. Metadata is fetched in parallel.

### Remove apps

```bash
rustore-fdroid -r ./repo remove <package_id> [package_id...]
```

Each app is removed from the working index immediately. Its APK, icon, and screenshots are removed after the next successful `sign`. Use `-k`/`--keep-files` to leave those files on disk.

### List apps

```bash
rustore-fdroid -r ./repo list
```

### Sign the repository

Generate a self-signed certificate (once):

```bash
openssl req -x509 -newkey rsa:4096 -keyout repo.key -out repo.crt -days 3650 -noenc -subj "/CN=My Repo"
```

Sign the index:

```bash
rustore-fdroid -r ./repo sign -c repo.crt -k repo.key
```

Generates and publishes:

- `index-v1.jar` for older and third-party clients
- `index-v2.json` with localized metadata and SHA-256 file metadata
- `entry.json` and a SHA-256-signed `entry.jar`, which authenticates the index v2 metadata

Both JAR files use the same certificate, so the repository fingerprint remains stable across index versions. After publishing the indexes, `sign` deletes superseded files queued by `add`, `update`, or `remove`. Diff files are not currently generated.

## Web Frontend

The frontend is embedded in the binary and can be managed with:

```bash
# install into repo (or use --frontend on init)
rustore-fdroid -r ./repo frontend add

# remove from repo
rustore-fdroid -r ./repo frontend remove
```

Point any HTTP server (Caddy, nginx, etc.) at the repo directory. The frontend reads `index-v1.json` and displays apps with search, screenshots, release notes, metadata, and APK download links. When `sign` runs with the frontend installed, it also publishes the repository certificate fingerprint, an F-Droid deep link, and a QR code. Static per-app pages under `packages/<package_id>/` support index v2 `webBaseUrl` share links without server rewrites.

## License

[GLWTS](LICENSE)
