# kguard

Go CLI to back up and restore Kafka SCRAM users and ACLs, storing backups in OCI Object Storage.

## Objectives

kguard helps Kafka administrators protect access configuration by backing up SCRAM users and ACLs into OCI Object Storage and restoring them when needed. Use it for disaster recovery, cluster migration, access auditing, or controlled replication of Kafka security settings across environments.

Backups do not store user passwords. During restore, kguard retrieves each user's password from OCI Vault, using secrets named after the Kafka usernames.

**For cross-region disaster recovery on OCI**:
  - Native Object Storage bucket replication for backup files, for more details check [Cross-Replication](https://docs.oracle.com/en/learn/object-replication-for-dr/#introduction)
  - Native OCI Vault secret replication for the required Kafka user passwords, for more details check [Vault Secrets Replication](https://docs.oracle.com/en-us/iaas/Content/secret-management/Tasks/configure-replication.htm)

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
- For creating users with `kguard user create`, the OCI Vault master encryption key OCID used to encrypt new secrets.
- OCI authentication configured with one of:
  - local `~/.oci/config`;
  - Instance Principal when running on an OCI instance; or
  - Cloud Shell with `OCI_CLI_AUTH=instance_obo_user`.

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

## Smoke Test

For a real integration test against a non-production Kafka/OCI environment, create the `profile-teste` profile first and run:

```bash
./scripts/smoke-test.sh
```

By default, the script uses the matching binary from `./dist`, such as `./dist/kguard-darwin-arm64`. Override with `KGUARD_BIN=/path/to/kguard` if needed. The script runs user, ACL, backup, restore validation, restore, and cleanup operations. It also validates `--from-json` for user and ACL creation. It does not create, update, or delete profiles.

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
./kguard profile update my-profile-dev
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
- OCI Object Storage backup prefix. The suggested default is the profile name.
- OCI region
- OCI Vault OCID
- OCI Vault master encryption key OCID
- OCI compartment OCID
- OCI auth mode: `OCI_CONFIG`, `INSTANCE_PRINCIPAL`, or `CLOUD_SHELL`
- OCI config profile
- Alternative OCI config file path

Use `profile update <profile>` to change an existing profile. Each prompt is pre-filled with the current value so you can keep or replace it.

When `OCI_CONFIG` is selected, kguard uses the configured OCI config file and profile. When `INSTANCE_PRINCIPAL` is selected, kguard uses OCI Instance Principal authentication and does not ask for an OCI config profile or file path. When `CLOUD_SHELL` is selected, kguard uses OCI Cloud Shell authentication through `OCI_CLI_AUTH=instance_obo_user` and the Cloud Shell delegation token, and also does not ask for an OCI config profile or file path.

Profile files are saved as:

```text
~/.kguard/profiles/<profile>
```

Configuration precedence is:

```text
flags > --from-json file > selected profile
```

Example JSON file:

```json
{
  "profile": "my-profile-dev",
  "bootstrap_servers": "broker1:9093,broker2:9093",
  "kafka_user": "admin",
  "kafka_password": "admin-password",
  "namespace": "my-namespace",
  "bucket": "kafka-acl-backup",
  "backup_prefix": "gru-cluster",
  "region": "sa-saopaulo-1",
  "vault_ocid": "ocid1.vault.oc1...",
  "vault_key_ocid": "ocid1.key.oc1...",
  "compartment_ocid": "ocid1.compartment.oc1...",
  "oci_auth_mode": "OCI_CONFIG"
}
```

Use it with:

```bash
./kguard backup --from-json ./kguard-input.json
```

JSON fields:

| Variable | Description | Context | Required |
| --- | --- | --- | --- |
| `profile` | kguard profile used as the last fallback layer. | All commands that load configuration: `backup`, `restore`, `user`, `acl` | N |
| `bootstrap_servers` | Comma-separated Kafka bootstrap brokers. | `backup`, `restore`, `user list`, `user create`, `user delete`, `acl list`, `acl create`, `acl delete` | Y for Kafka operations unless provided by flag/profile |
| `kafka_user` | Kafka admin user used by kguard to authenticate to Kafka. | `backup`, `restore`, `user list`, `user create`, `user delete`, `acl list`, `acl create`, `acl delete` | Y for Kafka operations unless provided by flag/profile |
| `kafka_password` | Kafka admin password used by kguard to authenticate to Kafka. | `backup`, `restore`, `user list`, `user create`, `user delete`, `acl list`, `acl create`, `acl delete` | Y for Kafka operations unless provided by flag/profile |
| `timeout` | Kafka/OCI operation timeout, for example `60s` or `1m`. | All commands that connect to Kafka or OCI | N |
| `namespace` | OCI Object Storage namespace. | `backup`, `restore` | Y for Object Storage operations unless provided by flag/profile |
| `bucket` | OCI Object Storage bucket used for backups. | `backup`, `restore` | Y for Object Storage operations unless provided by flag/profile |
| `backup_prefix` | OCI Object Storage prefix used when saving or resolving backup objects. | `backup`, `restore` | N |
| `region` | OCI region override, for example `sa-saopaulo-1`. | `backup`, `restore`, `user create`, `user delete` | N |
| `compartment_ocid` | OCI compartment OCID used to list/create Vault secrets. | `restore`, `user create`, `user delete` | Y for Vault operations unless provided by flag/profile |
| `vault_ocid` | OCI Vault OCID where Kafka user passwords are stored. | `restore`, `user create`, `user delete` | Y for Vault operations unless provided by flag/profile |
| `vault_key_ocid` | OCI Vault master encryption key OCID used to create new secrets. | `user create` | Y for `user create` unless provided by flag/profile |
| `oci_auth_mode` | OCI auth mode: `OCI_CONFIG`, `INSTANCE_PRINCIPAL`, or `CLOUD_SHELL`. | `backup`, `restore`, `user create`, `user delete` | N |
| `oci_profile` | Profile name inside the OCI config file. | OCI operations using `OCI_CONFIG` | N |
| `oci_config` | Alternative OCI config file path. | OCI operations using `OCI_CONFIG` | N |
| `object_name` | Backup object name to upload or restore. | `backup`, `restore` | N for `backup`; Y for non-interactive `restore` unless selecting interactively |
| `validate` | Restore validation-only mode. | `restore` | N |
| `force_password_creation` | During restore, creates missing OCI Vault secrets with generated random passwords before restoring Kafka users. Cannot be used with `validate`. | `restore` | N |
| `username` | Kafka SCRAM user name to create or delete. Explicit positional argument still has priority when provided. | `user create`, `user delete` | Y unless provided as positional argument |
| `password` | Password for the Kafka SCRAM user being created. | `user create` | Y for non-interactive `user create` unless provided by flag |
| `mechanism` | SCRAM mechanism, usually `SCRAM-SHA-512`. | `user create`, `user delete` | N |
| `iterations` | SCRAM iteration count. | `user create` | N |
| `topic` | Kafka topic resource pattern. Accepts a string or list. | `acl list`, `acl create`, `acl delete` | Y for topic ACLs |
| `group` | Kafka consumer group resource pattern. Accepts a string or list. | `acl list`, `acl create`, `acl delete` | Y for consumer ACLs |
| `cluster` | Whether to target the Kafka cluster resource. | `acl list`, `acl create`, `acl delete` | N |
| `transactional_id` | Kafka transactional ID resource pattern. Accepts a string or list. | `acl list`, `acl create`, `acl delete` | N |
| `delegation_token` | Kafka delegation token resource pattern. Accepts a string or list. | `acl list`, `acl create`, `acl delete` | N |
| `user_principal` | Kafka user resource pattern. Accepts a string or list. | `acl list`, `acl create`, `acl delete` | N |
| `resource_pattern_type` | ACL resource pattern type: `literal` or `prefixed`. | `acl list`, `acl create`, `acl delete` | N |
| `allow_principal` | Principal to allow, for example `User:app-user`. Accepts a string or list. | `acl create`, `acl delete` | Y for allow ACL changes |
| `deny_principal` | Principal to deny, for example `User:bad-user`. Accepts a string or list. | `acl create`, `acl delete` | Y for deny ACL changes |
| `allow_host` | Host for allowed principals. Accepts a string or list. Defaults to `*`. | `acl create`, `acl delete` | N |
| `deny_host` | Host for denied principals. Accepts a string or list. Defaults to `*`. | `acl create`, `acl delete` | N |
| `operation` | ACL operation such as `Read`, `Write`, `Describe`, or `All`. Accepts a string or list. | `acl create`, `acl delete` | N, defaults to `All` |
| `producer` | Enables Kafka producer ACL shortcut. | `acl create`, `acl delete` | N |
| `consumer` | Enables Kafka consumer ACL shortcut. | `acl create`, `acl delete` | N |
| `idempotent` | Adds idempotent producer ACL on the cluster resource. | `acl create`, `acl delete` | N |
| `principal` | Principal filter for listing ACLs. Accepts a string or list. | `acl list` | N |
| `force` | Assumes yes for supported confirmations. | `acl create`, `acl delete` | N |
| `interactive` | Enables or disables interactive prompts. | `backup`, `restore`, `user`, `acl` | N |
| `debug` | Shows detailed error messages, including full OCI SDK errors in validation output. | All commands | N |

If the selected profile does not exist, kguard asks you to initialize it with:

```bash
./kguard profile create my-profile-dev
```

The config files contain sensitive values such as Kafka passwords and are written with `0600` permissions.

The OCI Object Storage backup prefix is stored in the profile. The profile wizard suggests the profile name as the default, but you can change it.

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
  --bucket "my-bucket" \
  --backup-prefix "my-profile-dev"
```

By default, the backup is saved under the configured backup prefix with a name like:

```text
my-profile-dev/kafka-acl-backup-YYYYMMDDTHHMMSSZ.json
```

Users whose names start with `super-user` are ignored by default. ACLs whose principal matches `User:super-user*` are ignored too.

You can also set the object name:

```bash
./kguard backup \
  --object-name "prod-backup.json"
```

`--object-name` can be just the JSON file name. The configured backup prefix is applied automatically unless the value already starts with that prefix.

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

To force creation of missing Vault secrets during restore, kguard generates passwords in a UUID-like format and creates the missing secrets before restoring Kafka users:

```bash
./kguard restore --profile my-profile-dev --force-password-creation
```

Restore downloads the backup from Object Storage, reads passwords from OCI Vault, and recreates SCRAM users and ACLs in Kafka.

If `--object-name` and the profile's `OBJECT_NAME` are not set, the CLI lists all backup directories in the bucket, lets you choose a directory, and then lets you choose any `.json` backup file in that directory.

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

## Manage Users And ACLs

Create a Kafka SCRAM user and store the password as an OCI Vault secret with the same name:

```bash
./kguard user create app-user \
  --profile my-profile-dev \
  --password "change-me"
```

If the profile already exists, the JSON can contain only the values specific to the user and ACL operation. Example `create-user.json`:

```json
{
  "profile": "my-profile-dev",
  "username": "app-user",
  "password": "change-me",
  "mechanism": "SCRAM-SHA-512",
  "iterations": 4096
}
```

Create the user with:

```bash
./kguard user create --from-json ./create-user.json
```

Example `create-user-acl.json`:

```json
{
  "profile": "my-profile-dev",
  "allow_principal": "User:app-user",
  "allow_host": "*",
  "operation": "Read",
  "topic": "my-topic"
}
```

Create the ACL with:

```bash
./kguard acl create --from-json ./create-user-acl.json
```

Delete a Kafka SCRAM user and schedule deletion of the matching OCI Vault secret:

```bash
./kguard user delete app-user --profile my-profile-dev
```

List Kafka SCRAM users:

```bash
./kguard user list --profile my-profile-dev
```

Create, delete, and list ACLs:

```bash
./kguard acl create \
  --profile my-profile-dev \
  --allow-principal User:app-user \
  --allow-host '*' \
  --operation Read \
  --topic my-topic

./kguard acl delete \
  --profile my-profile-dev \
  --allow-principal User:app-user \
  --allow-host '*' \
  --operation Read \
  --topic my-topic

./kguard acl list --profile my-profile-dev --topic my-topic
```

The ACL flags follow the Kafka `kafka-acls.sh` style:

```bash
./kguard acl create --profile my-profile-dev --allow-principal User:app-user --producer --topic my-topic
./kguard acl create --profile my-profile-dev --allow-principal User:app-user --consumer --topic my-topic --group my-group
./kguard acl create --profile my-profile-dev --deny-principal User:bad-user --deny-host 198.51.100.3 --operation Read --topic my-topic
```

## Main Flags

- `--bootstrap-servers`: comma-separated Kafka brokers.
- `--kafka-user`: Kafka admin user.
- `--kafka-password`: Kafka admin password.
- `--from-json`: local JSON file used as an intermediate defaults layer between explicit flags and the selected profile.
- `--debug`: show detailed error messages.
- `--profile`: kguard configuration profile name.
- `--namespace`: OCI Object Storage namespace.
- `--bucket`: bucket used to save/read backups.
- `--backup-prefix`: OCI Object Storage backup prefix. The profile wizard suggests the profile name by default.
- `--region`: OCI region override.
- `--object-name`: backup JSON file name. The configured backup prefix is applied automatically unless this already includes it.
- `--validate`: validate Vault secrets for backup users without executing restore.
- `--force-password-creation`: create missing Vault secrets with generated random passwords during restore.
- `--vault-ocid`: Vault OCID used during restore.
- `--vault-key-ocid`: Vault master encryption key OCID used to create new password secrets.
- `--compartment-ocid`: compartment OCID containing the secrets.
- `--oci-auth-mode`: OCI authentication mode: `OCI_CONFIG`, `INSTANCE_PRINCIPAL`, or `CLOUD_SHELL`.
- `--oci-profile`: profile from `~/.oci/config`. Default: `DEFAULT`.
- `--oci-config`: alternative OCI config file path.
- `--timeout`: operation timeout. Default: `1m`.

## Notes

- Backups do not store Kafka user passwords.
- During restore, each password must exist in an OCI Vault secret with the same name as the Kafka user.
- OCI authentication is explicit per profile: `OCI_CONFIG`, `INSTANCE_PRINCIPAL`, or `CLOUD_SHELL`.

## License

This project is licensed under the MIT License. See [LICENSE](https://github.com/jctarla/kguard/blob/main/LICENSE).
