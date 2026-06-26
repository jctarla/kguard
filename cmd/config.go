package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	appconfig "kguard/internal/config"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var selectedProfile string

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func init() {
	rootCmd.PersistentFlags().StringVar(&selectedProfile, "profile", "", "kguard configuration profile name")
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileDeleteCmd)
}

var profileCmd = &cobra.Command{
	Use:           "profile",
	Short:         "Manage kguard configuration profiles",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Manage kguard configuration profiles.

Configuration profiles are stored in:
  ~/.kguard/profiles/<profile>

Examples:
  kguard profile create my-profile-dev
  kguard profile show my-profile-dev
  kguard profile list
  kguard profile delete my-profile-dev`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var profileCreateCmd = &cobra.Command{
	Use:           "create <profile>",
	Short:         "Create or update a kguard configuration profile",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("use `kguard profile create <profile>`")
		}
		if err := validateProfileName(args[0]); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigWizard(args[0])
	},
}

var profileShowCmd = &cobra.Command{
	Use:           "show <profile>",
	Short:         "Show a kguard configuration profile",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("use `kguard profile show <profile>`")
		}
		if err := validateProfileName(args[0]); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return showConfig(args[0])
	},
}

var profileListCmd = &cobra.Command{
	Use:           "list",
	Short:         "List kguard configuration profiles",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := listProfiles()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Println("No configuration profiles found.")
			return nil
		}
		for _, profile := range profiles {
			fmt.Println(profile)
		}
		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:           "delete <profile>",
	Short:         "Delete a kguard configuration profile",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("use `kguard profile delete <profile>`")
		}
		return validateProfileName(args[0])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return deleteProfile(args[0])
	},
}

func runConfigWizard(profile string) error {
	path, err := configFilePath(profile)
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
	values["BOOTSTRAP_SERVERS"], err = ask("Kafka bootstrap servers", "broker1:9093,broker2:9093", false)
	if err != nil {
		return err
	}
	values["KAFKA_USER"], err = ask("Kafka admin user", "", false)
	if err != nil {
		return err
	}
	values["KAFKA_PASSWORD"], err = ask("Kafka admin password", "", true)
	if err != nil {
		return err
	}
	values["NAMESPACE"], err = ask("OCI Object Storage namespace", "", false)
	if err != nil {
		return err
	}
	values["BUCKET"], err = ask("OCI Object Storage bucket", "", false)
	if err != nil {
		return err
	}
	values["REGION"], err = ask("OCI region", "sa-saopaulo-1", false)
	if err != nil {
		return err
	}
	values["OBJECT_NAME"] = ""
	values["VAULT_OCID"], err = ask("OCI Vault OCID", "", false)
	if err != nil {
		return err
	}
	values["COMPARTMENT_OCID"], err = ask("OCI Compartment OCID", "", false)
	if err != nil {
		return err
	}
	values["TIMEOUT"] = "60s"
	values["OCI_PROFILE"], err = ask("OCI config profile", "DEFAULT", false)
	if err != nil {
		return err
	}
	values["OCI_CONFIG"], err = askOptional("Alternative OCI config file path", "")
	if err != nil {
		return err
	}
	path, err = writeConfigFile(profile, values)
	if err != nil {
		return err
	}
	fmt.Printf("Configuration saved: %s\n", path)
	return nil
}

func showConfig(profile string) error {
	values, exists, err := readConfigFile(profile)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("configuration profile %q not found; run `kguard profile create %s`", profile, profile)
	}
	for _, key := range configKeys() {
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

func selectedConfigProfile() (string, error) {
	profile := strings.TrimSpace(selectedProfile)
	if profile == "" {
		return "", fmt.Errorf("provide --profile or create one with `kguard profile create <profile>`")
	}
	if err := validateProfileName(profile); err != nil {
		return "", err
	}
	return profile, nil
}

func applyConfigDefaults(profile string) (bool, error) {
	fileValues, fileExists, err := readConfigFile(profile)
	if err != nil {
		return false, err
	}
	applyConfigValues(fileValues)
	bs, _ := rootCmd.PersistentFlags().GetString("bootstrap-servers")
	kafkaFlags.BootstrapServers = appconfig.SplitCSV(bs)
	return fileExists, nil
}

func applyConfigValues(values map[string]string) {
	setFlagFromMap("bootstrap-servers", values, "BOOTSTRAP_SERVERS")
	setStringVarFromMap(&kafkaFlags.Username, "kafka-user", values, "KAFKA_USER")
	setStringVarFromMap(&kafkaFlags.Password, "kafka-password", values, "KAFKA_PASSWORD")
	setDurationVarFromMap(&kafkaFlags.Timeout, "timeout", values, "TIMEOUT")
	setStringVarFromMap(&ociFlags.Namespace, "namespace", values, "NAMESPACE")
	setStringVarFromMap(&ociFlags.Bucket, "bucket", values, "BUCKET")
	setStringVarFromMap(&ociFlags.Region, "region", values, "REGION")
	setStringVarFromMap(&ociFlags.CompartmentID, "compartment-ocid", values, "COMPARTMENT_OCID")
	setStringVarFromMap(&ociFlags.VaultID, "vault-ocid", values, "VAULT_OCID")
	setStringVarFromMap(&ociFlags.Profile, "oci-profile", values, "OCI_PROFILE")
	setStringVarFromMap(&ociFlags.ConfigPath, "oci-config", values, "OCI_CONFIG")
}

func applyProfileObjectName(profile string, flagChanged bool, target *string) error {
	if flagChanged || *target != "" {
		return nil
	}
	values, exists, err := readConfigFile(profile)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	*target = values["OBJECT_NAME"]
	return nil
}

func validateVaultConfig(profile string) error {
	if strings.TrimSpace(ociFlags.VaultID) == "" {
		return fmt.Errorf("provide the Vault OCID or run `kguard profile create %s`", profile)
	}
	if strings.TrimSpace(ociFlags.CompartmentID) == "" {
		return fmt.Errorf("provide the compartment OCID or run `kguard profile create %s`", profile)
	}
	return nil
}

func missingConfigError(profile string) error {
	return fmt.Errorf("configuration profile %q not found; run `kguard profile create %s`", profile, profile)
}

func hasConfigFlags() bool {
	for _, name := range []string{"bootstrap-servers", "kafka-user", "kafka-password", "namespace", "bucket", "region", "compartment-ocid", "vault-ocid", "oci-profile", "oci-config", "timeout"} {
		if rootCmd.PersistentFlags().Changed(name) {
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

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kguard", "profiles"), nil
}

func configFilePath(profile string) (string, error) {
	if err := validateProfileName(profile); err != nil {
		return "", err
	}
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, profile), nil
}

func writeConfigFile(profile string, values map[string]string) (string, error) {
	path, err := configFilePath(profile)
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
	for _, key := range configKeys() {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, strconv.Quote(values[key])); err != nil {
			return "", err
		}
	}
	return path, nil
}

func readConfigFile(profile string) (map[string]string, bool, error) {
	path, err := configFilePath(profile)
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

func listProfiles() ([]string, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	profiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if validateProfileName(name) == nil {
			profiles = append(profiles, name)
		}
	}
	sort.Strings(profiles)
	return profiles, nil
}

func deleteProfile(profile string) error {
	path, err := configFilePath(profile)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("configuration profile %q not found", profile)
		}
		return err
	}
	fmt.Printf("Configuration profile deleted: %s\n", profile)
	return nil
}

func validateProfileName(profile string) error {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile == "list" || profile == "delete" {
		return fmt.Errorf("%q is reserved and cannot be used as a profile name", profile)
	}
	if !profileNamePattern.MatchString(profile) {
		return fmt.Errorf("invalid profile name %q: use letters, numbers, dots, underscores, or hyphens", profile)
	}
	return nil
}

func configKeys() []string {
	return []string{
		"BOOTSTRAP_SERVERS",
		"KAFKA_USER",
		"KAFKA_PASSWORD",
		"NAMESPACE",
		"BUCKET",
		"REGION",
		"OBJECT_NAME",
		"VAULT_OCID",
		"COMPARTMENT_OCID",
		"TIMEOUT",
		"OCI_PROFILE",
		"OCI_CONFIG",
	}
}
