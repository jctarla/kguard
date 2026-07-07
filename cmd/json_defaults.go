package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var fromJSONValues map[string]any
var fromJSONLoaded bool

var jsonFlagAliases = map[string]string{
	"auth_mode":               "oci-auth-mode",
	"backup_prefix":           "backup-prefix",
	"bucket_backup_prefix":    "backup-prefix",
	"compartment_id":          "compartment-ocid",
	"compartment_ocid":        "compartment-ocid",
	"config":                  "oci-config",
	"kafka_admin_password":    "kafka-password",
	"kafka_admin_user":        "kafka-user",
	"object":                  "object-name",
	"object_name":             "object-name",
	"oci_auth_mode":           "oci-auth-mode",
	"oci_config":              "oci-config",
	"oci_profile":             "oci-profile",
	"scram_iterations":        "iterations",
	"scram_mechanism":         "mechanism",
	"vault_id":                "vault-ocid",
	"vault_key_id":            "vault-key-ocid",
	"vault_key_ocid":          "vault-key-ocid",
	"vault_ocid":              "vault-ocid",
	"resource_pattern_type":   "resource-pattern-type",
	"transactional_id":        "transactional-id",
	"transactional_ids":       "transactional-id",
	"delegation_token":        "delegation-token",
	"delegation_tokens":       "delegation-token",
	"user_principal":          "user-principal",
	"user_principals":         "user-principal",
	"allow_principal":         "allow-principal",
	"allow_principals":        "allow-principal",
	"deny_principal":          "deny-principal",
	"deny_principals":         "deny-principal",
	"allow_host":              "allow-host",
	"allow_hosts":             "allow-host",
	"deny_host":               "deny-host",
	"deny_hosts":              "deny-host",
	"debug_mode":              "debug",
	"operations":              "operation",
	"topics":                  "topic",
	"groups":                  "group",
	"user_create_password":    "password",
	"kafka_user_password":     "password",
	"force_delete":            "force",
	"force_password":          "force-password-creation",
	"force_password_creation": "force-password-creation",
}

func applyFromJSONDefaults(cmd *cobra.Command) error {
	if strings.TrimSpace(fromJSONPath) == "" {
		return nil
	}
	values, err := loadFromJSONValues()
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	for _, flags := range []*pflag.FlagSet{rootCmd.PersistentFlags(), cmd.InheritedFlags(), cmd.LocalFlags()} {
		if err := applyJSONToFlagSet(flags, values); err != nil {
			return err
		}
	}
	return nil
}

func loadFromJSONValues() (map[string]any, error) {
	if fromJSONLoaded {
		return fromJSONValues, nil
	}
	fromJSONLoaded = true
	path := strings.TrimSpace(fromJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --from-json file %q: %w", path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse --from-json file %q: %w", path, err)
	}
	fromJSONValues = normalizeJSONKeys(raw)
	return fromJSONValues, nil
}

func fromJSONValuesOrNil() map[string]any {
	if !fromJSONLoaded {
		return nil
	}
	return fromJSONValues
}

func stringFromJSON(key string) string {
	values := fromJSONValuesOrNil()
	if len(values) == 0 {
		return ""
	}
	value, ok := values[normalizeJSONKey(key)]
	if !ok {
		return ""
	}
	text, err := jsonScalarString(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

func normalizeJSONKeys(raw map[string]any) map[string]any {
	values := make(map[string]any, len(raw))
	for key, value := range raw {
		normalized := normalizeJSONKey(key)
		if alias, ok := jsonFlagAliases[normalized]; ok {
			values[normalizeJSONKey(alias)] = value
			continue
		}
		values[normalized] = value
	}
	return values
}

func applyJSONToFlagSet(flags *pflag.FlagSet, values map[string]any) error {
	if flags == nil {
		return nil
	}
	var err error
	flags.VisitAll(func(flag *pflag.Flag) {
		if err != nil || flag.Changed {
			return
		}
		value, ok := values[normalizeJSONKey(flag.Name)]
		if !ok {
			return
		}
		err = setFlagFromJSON(flag, value)
	})
	return err
}

func setFlagFromJSON(flag *pflag.Flag, value any) error {
	if value == nil {
		return nil
	}
	if values, ok := jsonStringList(value); ok {
		for _, item := range values {
			if err := flag.Value.Set(item); err != nil {
				return fmt.Errorf("set --%s from JSON: %w", flag.Name, err)
			}
		}
		return nil
	}
	text, err := jsonScalarString(value)
	if err != nil {
		return fmt.Errorf("set --%s from JSON: %w", flag.Name, err)
	}
	if text == "" {
		return nil
	}
	if err := flag.Value.Set(text); err != nil {
		return fmt.Errorf("set --%s from JSON: %w", flag.Name, err)
	}
	return nil
}

func jsonStringList(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, err := jsonScalarString(item)
		if err != nil || text == "" {
			continue
		}
		out = append(out, text)
	}
	return out, true
}

func jsonScalarString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}

func normalizeJSONKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, "-", "_")
	return key
}
