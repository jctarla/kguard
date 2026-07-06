package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"kguard/internal/config"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	kafkaFlags config.Kafka
	ociFlags   config.OCI
)

var appVersion = "dev"

var rootCmd = &cobra.Command{
	Use:           "kguard",
	Short:         "Back up and restore Kafka SCRAM users and ACLs to OCI Object Storage",
	Version:       appVersion,
	SilenceErrors: true,
	Long: `Interactive CLI to protect Kafka SCRAM users and ACLs.

Backup:
  - connects to Kafka using SASL_SSL with SCRAM-SHA-512
  - lists SCRAM users and ACLs
  - stores a versioned JSON file in OCI Object Storage

Restore:
  - downloads an existing backup from Object Storage
  - reads OCI Vault secrets named after each Kafka user
  - recreates SCRAM credentials and ACLs in the target cluster`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("kguard version {{.Version}}\n")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().String("bootstrap-servers", "", "Comma-separated Kafka bootstrap brokers. Example: b1:9093,b2:9093")
	rootCmd.PersistentFlags().StringVar(&kafkaFlags.Username, "kafka-user", "", "Kafka admin user for SCRAM-SHA-512 authentication")
	rootCmd.PersistentFlags().StringVar(&kafkaFlags.Password, "kafka-password", "", "Kafka admin password for SCRAM-SHA-512 authentication")
	rootCmd.PersistentFlags().DurationVar(&kafkaFlags.Timeout, "timeout", 60*time.Second, "Kafka/OCI operation timeout")
	rootCmd.PersistentFlags().StringVar(&ociFlags.Namespace, "namespace", "", "OCI Object Storage namespace")
	rootCmd.PersistentFlags().StringVar(&ociFlags.Bucket, "bucket", "", "OCI Object Storage bucket")
	rootCmd.PersistentFlags().StringVar(&ociFlags.Prefix, "backup-prefix", "", "OCI Object Storage bucket prefix used for backup objects")
	rootCmd.PersistentFlags().StringVar(&ociFlags.Region, "region", "", "OCI region override. Example: sa-saopaulo-1")
	rootCmd.PersistentFlags().StringVar(&ociFlags.CompartmentID, "compartment-ocid", "", "Compartment OCID used to list Vault secrets during restore")
	rootCmd.PersistentFlags().StringVar(&ociFlags.VaultID, "vault-ocid", "", "Vault OCID where Kafka user passwords are stored")
	rootCmd.PersistentFlags().StringVar(&ociFlags.AuthMode, "oci-auth-mode", "", "OCI authentication mode: OCI_CONFIG, INSTANCE_PRINCIPAL, or CLOUD_SHELL")
	rootCmd.PersistentFlags().StringVar(&ociFlags.Profile, "oci-profile", "DEFAULT", "Profile from ~/.oci/config")
	rootCmd.PersistentFlags().StringVar(&ociFlags.ConfigPath, "oci-config", "", "Alternative OCI config file path")
}

func printBanner() {
	fmt.Printf(` _                              _
| | ____ _ _   _  __ _ _ __ __| |
| |/ / _' | | | |/ _' | '__/ _' |
|   < (_| | |_| | (_| | | | (_| |
|_|\_\__, |\__,_|\__, |_|  \__,_|
     |___/       |___/

kguard %s - OCI-native Kafka access backup and restore with Vault and Object Storage
`, appVersion)
	fmt.Println()
}

func hydrateCommon(interactive bool) error {
	_ = interactive
	profile, err := selectedConfigProfile()
	if err != nil {
		return err
	}
	hasSource, err := applyConfigDefaults(profile)
	if err != nil {
		return err
	}
	if !hasSource {
		return missingConfigError(profile)
	}
	if err := config.ValidateKafka(kafkaFlags); err != nil {
		return err
	}
	if err := config.ValidateOCI(ociFlags); err != nil {
		return err
	}
	applyDefaultBackupPrefix(profile)
	return nil
}

func hydrateOCIOnly(interactive bool) error {
	_ = interactive
	profile, err := selectedConfigProfile()
	if err != nil {
		return err
	}
	hasSource, err := applyConfigDefaults(profile)
	if err != nil {
		return err
	}
	if !hasSource {
		return missingConfigError(profile)
	}
	if err := config.ValidateOCI(ociFlags); err != nil {
		return err
	}
	applyDefaultBackupPrefix(profile)
	return validateVaultConfig(profile)
}

func applyDefaultBackupPrefix(profile string) {
	if strings.TrimSpace(ociFlags.Prefix) == "" {
		ociFlags.Prefix = profile
	}
}

func ask(label, def string, secret bool) (string, error) {
	p := promptui.Prompt{Label: label, Default: def, Validate: nonEmpty}
	if secret {
		p.HideEntered = true
		p.Mask = '*'
	}
	return p.Run()
}

func askOptional(label, def string) (string, error) {
	p := promptui.Prompt{Label: label, Default: def, AllowEdit: true}
	return p.Run()
}

func askAuthMode(def string) (string, error) {
	options := []string{"OCI_CONFIG", "INSTANCE_PRINCIPAL", "CLOUD_SHELL"}
	start := 0
	for i, option := range options {
		if strings.EqualFold(def, option) {
			start = i
			break
		}
	}
	p := promptui.Select{
		Label:     "OCI auth mode",
		Items:     options,
		CursorPos: start,
	}
	_, value, err := p.Run()
	return value, err
}

func nonEmpty(v string) error {
	if v == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}
