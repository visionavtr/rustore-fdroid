# rustore-fdroid

CLI tool to generate and manage [F-Droid](https://f-droid.org/) repositories populated with apps from [RuStore](https://rustore.ru/). It publishes both index v1 and index v2, and includes a read-only web frontend for browsing and adding the repository.

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

All commands require `-r`/`--repo` flag pointing to the repository directory.

### Initialize a new repository

```bash
rustore-fdroid -r ./repo init -n "My Repo" -d "My F-Droid repository" -a "https://example.com/repo" --frontend
```

Use `--frontend` to include the web UI in the repo directory.

### Add apps

```bash
rustore-fdroid -r ./repo add <package_id> [package_id...]
```

Downloads APKs, icons, screenshots, changelogs, developer contacts, and compatibility metadata from RuStore. Supports multiple package IDs in one call; metadata is fetched in parallel. If an APK is already present and its xxhash matches, the download is skipped.

Only the latest APK for each app is retained.

### Update apps

```bash
rustore-fdroid -r ./repo update [package_id...]
```

Updates specified apps or all apps in the repository if no arguments given. Metadata is fetched in parallel.

### Remove apps

```bash
rustore-fdroid -r ./repo remove <package_id> [package_id...]
```

Use `-k`/`--keep-files` to keep the icon and APK files on disk.

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
- `index-v2.json` with localized metadata and verified media file hashes
- `entry.json` and SHA256withRSA-signed `entry.jar` as the index v2 trust anchor

The same certificate is used for v1 and v2, so the repository fingerprint remains stable. Diff files are not currently generated.

## Web Frontend

The frontend is embedded in the binary and can be managed with:

```bash
# install into repo (or use --frontend on init)
rustore-fdroid -r ./repo frontend add

# remove from repo
rustore-fdroid -r ./repo frontend remove
```

Point any HTTP server (Caddy, nginx, etc.) at the repo directory. The frontend reads `index-v1.json` and displays apps with search, screenshots, changelogs, metadata, and APK download links. After signing, it also displays the repository certificate fingerprint, an F-Droid deep link, and a QR code. Static per-app pages under `packages/<package_id>/` support index v2 `webBaseUrl` share links without server rewrites.

## License

[GLWTS](LICENSE)
