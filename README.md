# kguard

Go CLI to back up and restore Kafka SCRAM users and ACLs, storing backups in OCI Object Storage.

## TL;DR

Download the binary that matches your operating system and CPU architecture:

```bash
# Linux x64 / amd64
wget -O kguard https://github.com/jctarla/kguard/releases/latest/download/kguard-linux-x64
chmod +x kguard

# Linux arm64
wget -O kguard https://github.com/jctarla/kguard/releases/latest/download/kguard-linux-arm64
chmod +x kguard

# macOS Apple Silicon / arm64
wget -O kguard https://github.com/jctarla/kguard/releases/latest/download/kguard-darwin-arm64
chmod +x kguard

# macOS Intel / x64
wget -O kguard https://github.com/jctarla/kguard/releases/latest/download/kguard-darwin-x64
chmod +x kguard
```

Configure backup and restore:

```bash
./kguard config backup setup
./kguard config restore setup
```

Run backup:

```bash
./kguard backup
```

Validate a restore before applying it:

```bash
./kguard restore --validate
```

Run restore:

```bash
./kguard restore
```

## Requirements

- Go installed.
- Administrative access to the Kafka cluster.
- Kafka cluster using `SASL_SSL` with `SCRAM-SHA-512`.
- OCI Object Storage bucket.
- For restore, an OCI Vault secret for each Kafka user. The secret name must exactly match the Kafka username.
- OCI authentication configured with one of:
  - local `~/.oci/config`; or
  - Instance Principal when running on an OCI instance.

## Install

```bash
go build -o kguard .
```

Or download the latest binary from GitHub Releases:

```bash
# Linux x64 / amd64
wget -O kguard https://github.com/<owner>/kguard/releases/latest/download/kguard-linux-x64
chmod +x kguard

# Linux arm64
wget -O kguard https://github.com/<owner>/kguard/releases/latest/download/kguard-linux-arm64
chmod +x kguard

# macOS Apple Silicon / arm64
wget -O kguard https://github.com/<owner>/kguard/releases/latest/download/kguard-darwin-arm64
chmod +x kguard

# macOS Intel / x64
wget -O kguard https://github.com/<owner>/kguard/releases/latest/download/kguard-darwin-x64
chmod +x kguard
```

## Build Binaries

```bash
./scripts/build-x64.sh
./scripts/build-arm.sh
```

By default, the scripts build binaries for the current operating system. On macOS, for example:

```text
dist/kguard-darwin-x64
dist/kguard-darwin-arm64
```

To build Linux binaries from another system:

```bash
GOOS=linux ./scripts/build-x64.sh
GOOS=linux ./scripts/build-arm.sh
```

## Release

Releases are published by GitHub Actions when a version tag is pushed.

```bash
git tag v1.0.0
git push origin v1.0.0
```

The release workflow builds and uploads:

```text
kguard-linux-x64
kguard-linux-arm64
kguard-darwin-x64
kguard-darwin-arm64
checksums.txt
```

After the release is published, users can download a binary directly:

```bash
wget -O kguard https://github.com/<owner>/kguard/releases/latest/download/kguard-linux-x64
chmod +x kguard
./kguard --help
```

Use the binary that matches both the operating system and CPU architecture. For example, Apple Silicon Macs use `kguard-darwin-arm64`, not `kguard-linux-arm64`.

Replace `<owner>` with the GitHub user or organization that owns the repository.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).

## Help

```bash
./kguard --help
./kguard config --help
./kguard backup --help
./kguard restore --help
```

## Configure kguard

kguard can store per-operation configuration files in the user's home directory:

```bash
./kguard config backup setup
./kguard config restore setup
```

You can inspect current configuration with:

```bash
./kguard config backup show
./kguard config restore show
```

`show` masks password values as `*****`.

If a configuration file already exists, `setup` asks whether to overwrite it before changing anything.

`config backup setup` prompts for:

- Kafka bootstrap servers
- Kafka admin user
- Kafka admin password
- OCI Object Storage namespace
- OCI Object Storage bucket
- Object Storage prefix
- OCI region

`config restore setup` also prompts for:

- OCI Vault OCID
- OCI compartment OCID

The files are saved as:

```text
~/.kguard/backup
~/.kguard/restore
```

Configuration precedence is:

```text
flags > mode-specific environment variables > ~/.kguard/<mode>
```

If no config file exists, no mode-specific environment variables are set, and no relevant flags are passed, kguard asks you to initialize configuration with:

```bash
./kguard config backup setup
./kguard config restore setup
```

The config files contain sensitive values such as Kafka passwords and are written with `0600` permissions.

## Interactive Backup

```bash
./kguard backup
```

The CLI loads required values from flags, `KGUARD_BACKUP_*` environment variables, or `~/.kguard/backup`.

Backup always validates the collected users and ACLs before uploading the JSON file to Object Storage. The validation prints a user table and an ACL table; upload only happens if validation succeeds.

## Backup With Flags

```bash
./kguard backup \
  --bootstrap-servers "broker1:9093,broker2:9093" \
  --kafka-user "admin" \
  --kafka-password "admin-password" \
  --namespace "my-namespace" \
  --bucket "my-bucket" \
  --prefix "kafka-acl-backups"
```

By default, the backup is saved with a name like:

```text
kafka-acl-backups/kafka-acl-backup-YYYYMMDDTHHMMSSZ.json
```

Users whose names start with `super-user` are ignored by default. ACLs whose principal matches `User:super-user*` are ignored too.

You can also set the object name:

```bash
./kguard backup \
  --object-name "prod-backup.json"
```

`--object-name` can be just the JSON file name. The configured `--prefix` is applied automatically unless the value already starts with that prefix.

## Environment Variables

You can export mode-specific environment variables instead of passing flags or using config files. Flags always take precedence over environment variables.

For backup:

```bash
export KGUARD_BACKUP_BOOTSTRAP_SERVERS="broker1:9093,broker2:9093"
export KGUARD_BACKUP_KAFKA_USER="admin"
export KGUARD_BACKUP_KAFKA_PASSWORD="admin-password"
export KGUARD_BACKUP_NAMESPACE="my-namespace"
export KGUARD_BACKUP_BUCKET="my-bucket"
export KGUARD_BACKUP_PREFIX="kafka-acl-backups"
export KGUARD_BACKUP_REGION="sa-saopaulo-1"
export KGUARD_BACKUP_TIMEOUT="60s"

./kguard backup
```

For restore:

```bash
export KGUARD_RESTORE_BOOTSTRAP_SERVERS="broker1:9093,broker2:9093"
export KGUARD_RESTORE_KAFKA_USER="admin"
export KGUARD_RESTORE_KAFKA_PASSWORD="admin-password"
export KGUARD_RESTORE_NAMESPACE="my-namespace"
export KGUARD_RESTORE_BUCKET="my-bucket"
export KGUARD_RESTORE_PREFIX="kafka-acl-backups"
export KGUARD_RESTORE_REGION="us-ashburn-1"
export KGUARD_RESTORE_OBJECT_NAME="kafka-acl-backups/prod-backup.json"
export KGUARD_RESTORE_VAULT_OCID="ocid1.vault.oc1..."
export KGUARD_RESTORE_COMPARTMENT_OCID="ocid1.compartment.oc1..."

./kguard restore
```

## Validate Before Restore

Use `--validate` to download the backup, validate that every user has a password secret in OCI Vault, and print the user and ACL tables. No restore operation is executed.

```bash
./kguard restore --validate \
  --namespace "my-namespace" \
  --bucket "my-bucket" \
  --object-name "kafka-acl-backups/prod-backup.json" \
  --vault-ocid "ocid1.vault.oc1..." \
  --compartment-ocid "ocid1.compartment.oc1..."
```

Expected output:

```text
USER       SECRET     STATUS  MESSAGE
app-user   app-user   OK      user password found on OCI Vault
etl-user   etl-user   FAIL    secret "etl-user" was not found in the configured Vault
```

## Interactive Restore

```bash
./kguard restore
```

Restore downloads the backup from Object Storage, reads passwords from OCI Vault, and recreates SCRAM users and ACLs in Kafka.

If `--object-name` or `KGUARD_RESTORE_OBJECT_NAME` are not set, the CLI lists the 10 most recent `.json` backups from Object Storage under the configured prefix and lets you choose one.

Before applying restore, the CLI always runs the same validation as `--validate`: it shows the backup Kafka cluster, the target Kafka cluster, validates user passwords in Vault, and lists the ACLs that will be imported. If any password is missing or invalid, restore is not executed.

Restore behavior is deterministic:

- If a SCRAM user already exists, kguard updates that user's password with the value from OCI Vault.
- Existing ACLs for principals present in the backup are deleted from the target cluster before the backup ACLs are created.
- The final ACL set for those principals is the ACL set from the backup.

## Restore With Flags

```bash
./kguard restore \
  --bootstrap-servers "broker1:9093,broker2:9093" \
  --kafka-user "admin" \
  --kafka-password "admin-password" \
  --namespace "my-namespace" \
  --bucket "my-bucket" \
  --object-name "kafka-acl-backups/prod-backup.json" \
  --vault-ocid "ocid1.vault.oc1..." \
  --compartment-ocid "ocid1.compartment.oc1..."
```

In interactive mode, restore asks for confirmation with `Y` as the default:

```text
Apply restore to the target Kafka cluster? (Y/n)
```

## Main Flags

- `--bootstrap-servers`: comma-separated Kafka brokers.
- `--kafka-user`: Kafka admin user.
- `--kafka-password`: Kafka admin password.
- `--namespace`: OCI Object Storage namespace.
- `--bucket`: bucket used to save/read backups.
- `--prefix`: bucket prefix/folder. Default: `kafka-acl-backups`.
- `--region`: OCI region override. Backup and restore can use different regions through their mode-specific configs or environment variables.
- `--object-name`: backup JSON file name. The configured `--prefix` is applied automatically unless this already includes it.
- `--validate`: validate Vault secrets for backup users without executing restore.
- `--vault-ocid`: Vault OCID used during restore.
- `--compartment-ocid`: compartment OCID containing the secrets.
- `--oci-profile`: profile from `~/.oci/config`. Default: `DEFAULT`.
- `--oci-config`: alternative OCI config file path.
- `--timeout`: operation timeout. Default: `1m`.

## Environment Variable Reference

Use `KGUARD_BACKUP_*` for `kguard backup` and `KGUARD_RESTORE_*` for `kguard restore`.

- `KGUARD_<MODE>_BOOTSTRAP_SERVERS`: equivalent to `--bootstrap-servers`.
- `KGUARD_<MODE>_KAFKA_USER`: equivalent to `--kafka-user`.
- `KGUARD_<MODE>_KAFKA_PASSWORD`: equivalent to `--kafka-password`.
- `KGUARD_<MODE>_NAMESPACE`: equivalent to `--namespace`.
- `KGUARD_<MODE>_BUCKET`: equivalent to `--bucket`.
- `KGUARD_<MODE>_PREFIX`: equivalent to `--prefix`.
- `KGUARD_<MODE>_REGION`: equivalent to `--region`.
- `KGUARD_<MODE>_OBJECT_NAME`: equivalent to `--object-name`.
- `KGUARD_<MODE>_VAULT_OCID`: equivalent to `--vault-ocid`.
- `KGUARD_<MODE>_COMPARTMENT_OCID`: equivalent to `--compartment-ocid`.
- `KGUARD_<MODE>_OCI_PROFILE`: equivalent to `--oci-profile`.
- `KGUARD_<MODE>_OCI_CONFIG`: equivalent to `--oci-config`.
- `KGUARD_<MODE>_TIMEOUT`: equivalent to `--timeout`. Accepts values like `60s`, `2m`, or a number of seconds.

## Notes

- Backups do not store Kafka user passwords.
- During restore, each password must exist in an OCI Vault secret with the same name as the Kafka user.
- The CLI tries local OCI config first. If no valid config exists, it falls back to Instance Principal.
