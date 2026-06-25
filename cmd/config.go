package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appconfig "kguard/internal/config"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	configModeBackup  = "backup"
	configModeRestore = "restore"
)

var runtimeConfigMode string

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(newModeConfigCommand(configModeBackup))
	configCmd.AddCommand(newModeConfigCommand(configModeRestore))
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage kguard configuration",
	Long: `Manage kguard configuration files.

Configuration is stored per operation:
  - backup:  ~/.kguard/backup
  - restore: ~/.kguard/restore`,
}

func newModeConfigCommand(mode string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           mode,
		Short:         fmt.Sprintf("Manage %s configuration", mode),
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:           "setup",
		Short:         fmt.Sprintf("Create or update ~/.kguard/%s", mode),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigWizard(mode)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:           "show",
		Short:         fmt.Sprintf("Show ~/.kguard/%s", mode),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showConfig(mode)
		},
	})
	return cmd
}

func runConfigWizard(mode string) error {
	path, err := configFilePath(mode)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		overwrite, err := confirmOverwriteConfig(path)
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Println("Configuration unchanged.")
			return nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	values := map[string]string{}
	values[envKey(mode, "BOOTSTRAP_SERVERS")], err = ask("Kafka bootstrap servers", "broker1:9093,broker2:9093", false)
	if err != nil {
		return err
	}
	values[envKey(mode, "KAFKA_USER")], err = ask("Kafka admin user", "", false)
	if err != nil {
		return err
	}
	values[envKey(mode, "KAFKA_PASSWORD")], err = ask("Kafka admin password", "", true)
	if err != nil {
		return err
	}
	values[envKey(mode, "NAMESPACE")], err = ask("OCI Object Storage namespace", "", false)
	if err != nil {
		return err
	}
	values[envKey(mode, "BUCKET")], err = ask("OCI Object Storage bucket", "", false)
	if err != nil {
		return err
	}
	values[envKey(mode, "PREFIX")], err = ask("OCI Object Storage prefix", "kafka-acl-backups", false)
	if err != nil {
		return err
	}
	values[envKey(mode, "REGION")], err = ask("OCI region", "sa-saopaulo-1", false)
	if err != nil {
		return err
	}
	if mode == configModeRestore {
		values[envKey(mode, "VAULT_OCID")], err = ask("OCI Vault OCID", "", false)
		if err != nil {
			return err
		}
		values[envKey(mode, "COMPARTMENT_OCID")], err = ask("OCI Compartment OCID", "", false)
		if err != nil {
			return err
		}
	}
	values[envKey(mode, "TIMEOUT")] = "60s"
	values[envKey(mode, "OCI_PROFILE")] = "DEFAULT"
	path, err = writeConfigFile(mode, values)
	if err != nil {
		return err
	}
	fmt.Printf("Configuration saved: %s\n", path)
	return nil
}

func showConfig(mode string) error {
	values, exists, err := readConfigFile(mode)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("configuration file not found; run `kguard config %s setup`", mode)
	}
	keys := configKeys(mode)
	for _, key := range keys {
		value := values[key]
		if strings.Contains(key, "PASSWORD") && value != "" {
			value = "*****"
		}
		fmt.Printf("%s=%s\n", key, strconv.Quote(value))
	}
	return nil
}

func confirmOverwriteConfig(path string) (bool, error) {
	prompt := promptui.Prompt{
		Label:   fmt.Sprintf("Configuration file %s already exists. Overwrite? (y/N)", path),
		Default: "N",
		Validate: func(v string) error {
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "", "y", "n":
				return nil
			default:
				return fmt.Errorf("answer Y for yes or N for no")
			}
		},
	}
	answer, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(answer)) == "y", nil
}

func activeConfigMode() string {
	if runtimeConfigMode != "" {
		return runtimeConfigMode
	}
	return configModeBackup
}

func applyConfigDefaults(mode string) (bool, error) {
	fileValues, fileExists, err := readConfigFile(mode)
	if err != nil {
		return false, err
	}
	hasEnv := hasModeEnv(mode)
	hasFlags := hasConfigFlags()
	applyConfigValues(mode, fileValues)
	if err := applyModeEnv(mode); err != nil {
		return false, err
	}
	bs, _ := rootCmd.PersistentFlags().GetString("bootstrap-servers")
	kafkaFlags.BootstrapServers = appconfig.SplitCSV(bs)
	return fileExists || hasEnv || hasFlags, nil
}

func applyConfigValues(mode string, values map[string]string) {
	setFlagFromMap("bootstrap-servers", values, envKey(mode, "BOOTSTRAP_SERVERS"))
	setStringVarFromMap(&kafkaFlags.Username, "kafka-user", values, envKey(mode, "KAFKA_USER"))
	setStringVarFromMap(&kafkaFlags.Password, "kafka-password", values, envKey(mode, "KAFKA_PASSWORD"))
	setDurationVarFromMap(&kafkaFlags.Timeout, "timeout", values, envKey(mode, "TIMEOUT"))
	setStringVarFromMap(&ociFlags.Namespace, "namespace", values, envKey(mode, "NAMESPACE"))
	setStringVarFromMap(&ociFlags.Bucket, "bucket", values, envKey(mode, "BUCKET"))
	setStringVarFromMap(&ociFlags.Prefix, "prefix", values, envKey(mode, "PREFIX"))
	setStringVarFromMap(&ociFlags.Region, "region", values, envKey(mode, "REGION"))
	setStringVarFromMap(&ociFlags.CompartmentID, "compartment-ocid", values, envKey(mode, "COMPARTMENT_OCID"))
	setStringVarFromMap(&ociFlags.VaultID, "vault-ocid", values, envKey(mode, "VAULT_OCID"))
	setStringVarFromMap(&ociFlags.Profile, "oci-profile", values, envKey(mode, "OCI_PROFILE"))
	setStringVarFromMap(&ociFlags.ConfigPath, "oci-config", values, envKey(mode, "OCI_CONFIG"))
}

func applyModeEnv(mode string) error {
	setFlagFromEnv("bootstrap-servers", envKey(mode, "BOOTSTRAP_SERVERS"))
	setStringVarFromEnv(&kafkaFlags.Username, "kafka-user", envKey(mode, "KAFKA_USER"))
	setStringVarFromEnv(&kafkaFlags.Password, "kafka-password", envKey(mode, "KAFKA_PASSWORD"))
	if err := setDurationVarFromEnv(&kafkaFlags.Timeout, "timeout", envKey(mode, "TIMEOUT")); err != nil {
		return err
	}
	setStringVarFromEnv(&ociFlags.Namespace, "namespace", envKey(mode, "NAMESPACE"))
	setStringVarFromEnv(&ociFlags.Bucket, "bucket", envKey(mode, "BUCKET"))
	setStringVarFromEnv(&ociFlags.Prefix, "prefix", envKey(mode, "PREFIX"))
	setStringVarFromEnv(&ociFlags.Region, "region", envKey(mode, "REGION"))
	setStringVarFromEnv(&ociFlags.CompartmentID, "compartment-ocid", envKey(mode, "COMPARTMENT_OCID"))
	setStringVarFromEnv(&ociFlags.VaultID, "vault-ocid", envKey(mode, "VAULT_OCID"))
	setStringVarFromEnv(&ociFlags.Profile, "oci-profile", envKey(mode, "OCI_PROFILE"))
	setStringVarFromEnv(&ociFlags.ConfigPath, "oci-config", envKey(mode, "OCI_CONFIG"))
	return nil
}

func validateVaultConfig() error {
	if strings.TrimSpace(ociFlags.VaultID) == "" {
		return fmt.Errorf("provide the Vault OCID or run `kguard config %s setup`", activeConfigMode())
	}
	if strings.TrimSpace(ociFlags.CompartmentID) == "" {
		return fmt.Errorf("provide the compartment OCID or run `kguard config %s setup`", activeConfigMode())
	}
	return nil
}

func missingConfigError(mode string) error {
	return fmt.Errorf("no %s configuration found; run `kguard config %s setup`, set %s_* environment variables, or pass flags", mode, mode, envPrefix(mode))
}

func hasConfigFlags() bool {
	for _, name := range []string{"bootstrap-servers", "kafka-user", "kafka-password", "namespace", "bucket", "prefix", "region", "compartment-ocid", "vault-ocid", "oci-profile", "oci-config", "timeout"} {
		if rootCmd.PersistentFlags().Changed(name) {
			return true
		}
	}
	return false
}

func hasModeEnv(mode string) bool {
	prefix := envPrefix(mode) + "_"
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			return true
		}
	}
	return false
}

func setFlagFromMap(flagName string, values map[string]string, key string) {
	if rootCmd.PersistentFlags().Changed(flagName) {
		return
	}
	if value := values[key]; value != "" {
		_ = rootCmd.PersistentFlags().Set(flagName, value)
	}
}

func setStringVarFromMap(target *string, flagName string, values map[string]string, key string) {
	if rootCmd.PersistentFlags().Changed(flagName) {
		return
	}
	if value := values[key]; value != "" {
		*target = value
	}
}

func setDurationVarFromMap(target *time.Duration, flagName string, values map[string]string, key string) {
	if rootCmd.PersistentFlags().Changed(flagName) {
		return
	}
	if value := values[key]; value != "" {
		if duration, err := parseDuration(value); err == nil {
			*target = duration
		}
	}
}

func setFlagFromEnv(flagName, envName string) {
	if rootCmd.PersistentFlags().Changed(flagName) {
		return
	}
	if value := os.Getenv(envName); value != "" {
		_ = rootCmd.PersistentFlags().Set(flagName, value)
	}
}

func setStringVarFromEnv(target *string, flagName, envName string) {
	if rootCmd.PersistentFlags().Changed(flagName) {
		return
	}
	if value := os.Getenv(envName); value != "" {
		*target = value
	}
}

func setDurationVarFromEnv(target *time.Duration, flagName, envName string) error {
	if rootCmd.PersistentFlags().Changed(flagName) {
		return nil
	}
	value := os.Getenv(envName)
	if value == "" {
		return nil
	}
	duration, err := parseDuration(value)
	if err != nil {
		return fmt.Errorf("%s is invalid: use a Go duration like 60s or 2m, or a number of seconds", envName)
	}
	*target = duration
	return nil
}

func parseDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		seconds, convErr := strconv.Atoi(value)
		if convErr != nil {
			return 0, err
		}
		duration = time.Duration(seconds) * time.Second
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero")
	}
	return duration, nil
}

func configFilePath(mode string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kguard", mode), nil
}

func writeConfigFile(mode string, values map[string]string) (string, error) {
	path, err := configFilePath(mode)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	for _, key := range configKeys(mode) {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, strconv.Quote(values[key])); err != nil {
			return "", err
		}
	}
	return path, nil
}

func readConfigFile(mode string) (map[string]string, bool, error) {
	path, err := configFilePath(mode)
	if err != nil {
		return nil, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, false, nil
		}
		return nil, false, err
	}
	defer file.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	return values, true, nil
}

func envPrefix(mode string) string {
	switch mode {
	case configModeRestore:
		return "KGUARD_RESTORE"
	default:
		return "KGUARD_BACKUP"
	}
}

func envKey(mode, suffix string) string {
	return envPrefix(mode) + "_" + suffix
}

func configKeys(mode string) []string {
	keys := []string{
		envKey(mode, "BOOTSTRAP_SERVERS"),
		envKey(mode, "KAFKA_USER"),
		envKey(mode, "KAFKA_PASSWORD"),
		envKey(mode, "NAMESPACE"),
		envKey(mode, "BUCKET"),
		envKey(mode, "PREFIX"),
		envKey(mode, "REGION"),
	}
	if mode == configModeRestore {
		keys = append(keys, envKey(mode, "VAULT_OCID"), envKey(mode, "COMPARTMENT_OCID"))
	}
	return append(keys, envKey(mode, "TIMEOUT"), envKey(mode, "OCI_PROFILE"), envKey(mode, "OCI_CONFIG"))
}
