# kguard

Go CLI to back up and restore Kafka SCRAM users and ACLs, storing backups in OCI Object Storage.

## Objectives

kguard helps Kafka administrators protect access configuration by backing up SCRAM users and ACLs into OCI Object Storage and restoring them when needed. Use it for disaster recovery, cluster migration, access auditing, or controlled replication of Kafka security settings across environments.

Backups do not store user passwords. During restore, kguard retrieves each user's password from OCI Vault, using secrets named after the Kafka usernames.

For cross-region disaster recovery on OCI, use native Object Storage bucket replication for backup files and native OCI Vault secret replication for the required Kafka user passwords.

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
./kguard profile create my-profile-dev
```

Run backup:

```bash
./kguard backup --profile my-profile-dev
```

Validate a restore before applying it:

```bash
./kguard restore --profile my-profile-dev --validate
```

Run restore:

```bash
./kguard restore --profile my-profile-dev
```

## Requirements

- Go installed (if you plan to build the binary)
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

## Help

```bash
./kguard --help
./kguard profile --help
./kguard backup --help
./kguard restore --help
```

## Configure profiles

kguard stores named configuration profiles in the user's home directory. A profile contains all values needed by both backup and restore operations.

```bash
./kguard profile create my-profile-dev
```

You can inspect, list, or delete profiles with:

```bash
./kguard profile show my-profile-dev
./kguard profile list
./kguard profile delete my-profile-dev
```

`show` masks password values as `*****`.

If a configuration file already exists, `profile create` asks whether to overwrite it before changing anything.

`profile create <profile>` prompts for:

- Kafka bootstrap servers
- Kafka admin user
- Kafka admin password
- OCI Object Storage namespace
- OCI Object Storage bucket
- OCI region
- OCI Vault OCID
- OCI compartment OCID
- OCI config profile
- Alternative OCI config file path

Profile files are saved as:

```text
~/.kguard/profiles/<profile>
```

Configuration precedence is:

```text
flags > selected profile
```

If the selected profile does not exist, kguard asks you to initialize it with:

```bash
./kguard profile create my-profile-dev
```

The config files contain sensitive values such as Kafka passwords and are written with `0600` permissions.

The OCI Object Storage prefix is always the selected profile name. For example, `--profile my-profile-dev` stores and reads backup objects under the `my-profile-dev/` prefix.

## Interactive Backup

```bash
./kguard backup --profile my-profile-dev
```

The CLI loads required values from flags or the selected profile.

Backup always validates the collected users and ACLs before uploading the JSON file to Object Storage. The validation prints a user table and an ACL table; upload only happens if validation succeeds.

## Backup With Flags

```bash
./kguard backup \
  --profile "my-profile-dev" \
  --bootstrap-servers "broker1:9093,broker2:9093" \
  --kafka-user "admin" \
  --kafka-password "admin-password" \
  --namespace "my-namespace" \
  --bucket "my-bucket"
```

By default, the backup is saved with a name like:

```text
my-profile-dev/kafka-acl-backup-YYYYMMDDTHHMMSSZ.json
```

Users whose names start with `super-user` are ignored by default. ACLs whose principal matches `User:super-user*` are ignored too.

You can also set the object name:

```bash
./kguard backup \
  --object-name "prod-backup.json"
```

`--object-name` can be just the JSON file name. The selected profile name is applied automatically as the Object Storage prefix unless the value already starts with that profile prefix.

## Validate Before Restore

Use `--validate` to download the backup, validate that every user has a password secret in OCI Vault, and print the user and ACL tables. No restore operation is executed.

```bash
./kguard restore --validate \
  --profile "my-profile-dev" \
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
./kguard restore --profile my-profile-dev
```

Restore downloads the backup from Object Storage, reads passwords from OCI Vault, and recreates SCRAM users and ACLs in Kafka.

If `--object-name` and the profile's `OBJECT_NAME` are not set, the CLI lists the 10 most recent `.json` backups from Object Storage under the selected profile prefix and lets you choose one.

Before applying restore, the CLI always runs the same validation as `--validate`: it shows the backup Kafka cluster, the target Kafka cluster, validates user passwords in Vault, and lists the ACLs that will be imported. If any password is missing or invalid, restore is not executed.

Restore behavior is deterministic:

- If a SCRAM user already exists, kguard updates that user's password with the value from OCI Vault.
- Existing ACLs for principals present in the backup are deleted from the target cluster before the backup ACLs are created.
- The final ACL set for those principals is the ACL set from the backup.

## Restore With Flags

```bash
./kguard restore \
  --profile "my-profile-dev" \
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
- `--profile`: kguard configuration profile name.
- `--namespace`: OCI Object Storage namespace.
- `--bucket`: bucket used to save/read backups.
- `--region`: OCI region override.
- `--object-name`: backup JSON file name. The selected profile name is used as the Object Storage prefix.
- `--validate`: validate Vault secrets for backup users without executing restore.
- `--vault-ocid`: Vault OCID used during restore.
- `--compartment-ocid`: compartment OCID containing the secrets.
- `--oci-profile`: profile from `~/.oci/config`. Default: `DEFAULT`.
- `--oci-config`: alternative OCI config file path.
- `--timeout`: operation timeout. Default: `1m`.

## Notes

- Backups do not store Kafka user passwords.
- During restore, each password must exist in an OCI Vault secret with the same name as the Kafka user.
- The CLI tries local OCI config first. If no valid config exists, it falls back to Instance Principal.

## License

This project is licensed under the MIT License. See [LICENSE](https://github.com/jctarla/kguard/blob/main/LICENSE).
