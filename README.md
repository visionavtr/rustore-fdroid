# rustore-fdroid

CLI tool to generate and manage [F-Droid](https://f-droid.org/) repositories populated with apps from [RuStore](https://rustore.ru/).

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
rustore-fdroid -r ./repo init -n "My Repo" -d "My F-Droid repository" -a "https://example.com/repo"
```

### Add an app

```bash
rustore-fdroid -r ./repo add <package_id>
```

Downloads the icon and APK from RuStore and adds the app to the index. If the APK is already present and its xxhash matches, the download is skipped.

### Remove an app

```bash
rustore-fdroid -r ./repo remove <package_id>
```

Use `-k`/`--keep-files` to keep the icon and APK files on disk.

### List apps

```bash
rustore-fdroid -r ./repo list
```

### Sign the repository

```bash
rustore-fdroid -r ./repo sign -c cert.pem -k key.pem
```

Generates `index-v1.jar` with JAR signature (MANIFEST.MF + CERT.SF + PKCS7).

## License

[GLWTS](LICENSE)
