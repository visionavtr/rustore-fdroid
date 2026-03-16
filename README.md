# rustore-fdroid

CLI tool to generate and manage [F-Droid](https://f-droid.org/) repositories populated with apps from [RuStore](https://rustore.ru/). Includes a read-only web frontend for browsing the repository.

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

Downloads icons and APKs from RuStore and adds apps to the index. Supports multiple package IDs in one call — metadata is fetched in parallel. If an APK is already present and its xxhash matches, the download is skipped.

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

Generates `index-v1.jar` with JAR signature (MANIFEST.MF + CERT.SF + PKCS7).

## Web Frontend

The frontend is embedded in the binary and can be managed with:

```bash
# install into repo (or use --frontend on init)
rustore-fdroid -r ./repo frontend add

# remove from repo
rustore-fdroid -r ./repo frontend remove
```

Point any HTTP server (Caddy, nginx, etc.) at the repo directory — the frontend reads `index-v1.json` and displays apps with search, metadata, version history, and APK download links.

## License

[GLWTS](LICENSE)
