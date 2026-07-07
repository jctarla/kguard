#!/usr/bin/env bash

# kguard smoke/integration test for a pre-existing test profile.
# This script intentionally avoids "set -euo pipefail" so failures remain visible.

default_kguard_bin() {
  local os_name
  local arch_name

  os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch_name="$(uname -m)"

  case "$os_name" in
    darwin) os_name="darwin" ;;
    linux) os_name="linux" ;;
  esac

  case "$arch_name" in
    arm64|aarch64) arch_name="arm64" ;;
    x86_64|amd64) arch_name="x64" ;;
  esac

  printf './dist/kguard-%s-%s' "$os_name" "$arch_name"
}

KGUARD_BIN="${KGUARD_BIN:-$(default_kguard_bin)}"
KGUARD_PROFILE="${KGUARD_PROFILE:-profile-teste}"
KGUARD_TEST_USER="${KGUARD_TEST_USER:-kguard-smoke-user}"
KGUARD_TEST_PASSWORD="${KGUARD_TEST_PASSWORD:-KguardSmokePassword-123!}"
KGUARD_TEST_TOPIC="${KGUARD_TEST_TOPIC:-kguard-smoke-topic}"
KGUARD_TEST_GROUP="${KGUARD_TEST_GROUP:-kguard-smoke-group}"
KGUARD_TEST_HOST="${KGUARD_TEST_HOST:-*}"
KGUARD_TEST_OPERATION="${KGUARD_TEST_OPERATION:-Read}"
KGUARD_TEST_BACKUP="${KGUARD_TEST_BACKUP:-kguard-smoke-backup-$(date -u +%Y%m%dT%H%M%SZ).json}"
KGUARD_SMOKE_DIR="${KGUARD_SMOKE_DIR:-/tmp/kguard-smoke}"
KGUARD_USER_JSON="${KGUARD_SMOKE_DIR}/create-user.json"
KGUARD_ACL_JSON="${KGUARD_SMOKE_DIR}/create-acl.json"

PASSED_STEPS=()
FAILED_STEPS=()
CREATED_USER="N"
CREATED_ACL="N"

print_header() {
  printf '\n== %s ==\n' "$1"
}

record_pass() {
  PASSED_STEPS+=("$1")
  printf 'PASS: %s\n' "$1"
}

record_fail() {
  FAILED_STEPS+=("$1")
  printf 'FAIL: %s\n' "$1"
}

run_step() {
  local name="$1"
  shift

  print_header "$name"
  printf '+'
  printf ' %q' "$@"
  printf '\n'

  "$@"
  local status=$?
  if [ "$status" -eq 0 ]; then
    record_pass "$name"
    return 0
  fi

  record_fail "$name"
  printf 'Command failed with exit code %s\n' "$status"
  return "$status"
}

run_cleanup_step() {
  local name="$1"
  shift

  print_header "$name"
  printf '+'
  printf ' %q' "$@"
  printf '\n'

  "$@"
  local status=$?
  if [ "$status" -eq 0 ]; then
    printf 'Cleanup OK: %s\n' "$name"
  else
    printf 'Cleanup failed with exit code %s: %s\n' "$status" "$name"
  fi
}

fail_and_cleanup() {
  printf '\nSmoke test stopped after a failure.\n'
  cleanup
  summary
  exit 1
}

cleanup() {
  print_header "Cleanup"

  if [ "$CREATED_ACL" = "Y" ]; then
    run_cleanup_step "delete smoke ACL" \
      "$KGUARD_BIN" acl delete \
        --profile "$KGUARD_PROFILE" \
        --allow-principal "User:$KGUARD_TEST_USER" \
        --allow-host "$KGUARD_TEST_HOST" \
        --operation "$KGUARD_TEST_OPERATION" \
        --topic "$KGUARD_TEST_TOPIC" \
        --interactive=false
  else
    printf 'Skipping ACL cleanup; ACL was not marked as created.\n'
  fi

  if [ "$CREATED_USER" = "Y" ]; then
    run_cleanup_step "delete smoke user" \
      "$KGUARD_BIN" user delete "$KGUARD_TEST_USER" \
        --profile "$KGUARD_PROFILE" \
        --interactive=false
  else
    printf 'Skipping user cleanup; user was not marked as created.\n'
  fi
}

summary() {
  print_header "Summary"

  printf 'Profile: %s\n' "$KGUARD_PROFILE"
  printf 'User: %s\n' "$KGUARD_TEST_USER"
  printf 'Topic: %s\n' "$KGUARD_TEST_TOPIC"
  printf 'Group: %s\n' "$KGUARD_TEST_GROUP"
  printf 'Backup object: %s\n' "$KGUARD_TEST_BACKUP"

  printf '\nPassed steps (%s):\n' "${#PASSED_STEPS[@]}"
  for step in "${PASSED_STEPS[@]}"; do
    printf '  - %s\n' "$step"
  done

  printf '\nFailed steps (%s):\n' "${#FAILED_STEPS[@]}"
  for step in "${FAILED_STEPS[@]}"; do
    printf '  - %s\n' "$step"
  done
}

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

write_json_inputs() {
  mkdir -p "$KGUARD_SMOKE_DIR"

  cat > "$KGUARD_USER_JSON" <<EOF
{
  "profile": "$(json_escape "$KGUARD_PROFILE")",
  "username": "$(json_escape "$KGUARD_TEST_USER")",
  "password": "$(json_escape "$KGUARD_TEST_PASSWORD")",
  "mechanism": "SCRAM-SHA-512",
  "iterations": 4096,
  "interactive": false
}
EOF

  cat > "$KGUARD_ACL_JSON" <<EOF
{
  "profile": "$(json_escape "$KGUARD_PROFILE")",
  "allow_principal": "User:$(json_escape "$KGUARD_TEST_USER")",
  "allow_host": "$(json_escape "$KGUARD_TEST_HOST")",
  "operation": "$(json_escape "$KGUARD_TEST_OPERATION")",
  "topic": "$(json_escape "$KGUARD_TEST_TOPIC")",
  "interactive": false
}
EOF
}

print_header "kguard smoke test"
printf 'Using binary: %s\n' "$KGUARD_BIN"
printf 'Using profile: %s\n' "$KGUARD_PROFILE"
printf 'Using JSON dir: %s\n' "$KGUARD_SMOKE_DIR"

run_step "check kguard binary" "$KGUARD_BIN" --help || fail_and_cleanup
run_step "check existing profile" "$KGUARD_BIN" profile show "$KGUARD_PROFILE" || fail_and_cleanup
run_step "write from-json inputs" write_json_inputs || fail_and_cleanup

run_step "create smoke user from JSON" \
  "$KGUARD_BIN" user create \
    --from-json "$KGUARD_USER_JSON" || fail_and_cleanup
CREATED_USER="Y"

run_step "create smoke ACL from JSON" \
  "$KGUARD_BIN" acl create \
    --from-json "$KGUARD_ACL_JSON" || fail_and_cleanup
CREATED_ACL="Y"

run_step "list users" "$KGUARD_BIN" user list --profile "$KGUARD_PROFILE" --interactive=false || fail_and_cleanup

run_step "list ACLs" \
  "$KGUARD_BIN" acl list \
    --profile "$KGUARD_PROFILE" \
    --topic "$KGUARD_TEST_TOPIC" \
    --principal "User:$KGUARD_TEST_USER" \
    --interactive=false || fail_and_cleanup

run_step "backup" \
  "$KGUARD_BIN" backup \
    --profile "$KGUARD_PROFILE" \
    --object-name "$KGUARD_TEST_BACKUP" \
    --interactive=false || fail_and_cleanup

run_step "restore validate" \
  "$KGUARD_BIN" restore \
    --profile "$KGUARD_PROFILE" \
    --object-name "$KGUARD_TEST_BACKUP" \
    --validate \
    --interactive=false || fail_and_cleanup

run_step "restore" \
  "$KGUARD_BIN" restore \
    --profile "$KGUARD_PROFILE" \
    --object-name "$KGUARD_TEST_BACKUP" \
    --interactive=false || fail_and_cleanup

run_step "delete smoke ACL" \
  "$KGUARD_BIN" acl delete \
    --profile "$KGUARD_PROFILE" \
    --allow-principal "User:$KGUARD_TEST_USER" \
    --allow-host "$KGUARD_TEST_HOST" \
    --operation "$KGUARD_TEST_OPERATION" \
    --topic "$KGUARD_TEST_TOPIC" \
    --interactive=false || fail_and_cleanup
CREATED_ACL="N"

run_step "delete smoke user" \
  "$KGUARD_BIN" user delete "$KGUARD_TEST_USER" \
    --profile "$KGUARD_PROFILE" \
    --interactive=false || fail_and_cleanup
CREATED_USER="N"

summary

if [ "${#FAILED_STEPS[@]}" -eq 0 ]; then
  printf '\nSmoke test completed successfully.\n'
  exit 0
fi

printf '\nSmoke test completed with failures.\n'
exit 1
