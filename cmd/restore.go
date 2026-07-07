package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kguard/internal/backup"
	"kguard/internal/config"
	kafkaadmin "kguard/internal/kafka"
	ociclient "kguard/internal/oci"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var restoreObjectName string
var restoreValidateOnly bool

type restoreValidationResult struct {
	Backup    *backup.File
	Users     []validateUserRow
	Passwords map[string]string
}

var restoreCmd = &cobra.Command{
	Use:           "restore",
	Short:         "Restore SCRAM users and ACLs from an OCI Object Storage backup",
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		profile, err := selectedConfigProfile()
		if err != nil {
			return err
		}
		if restoreValidateOnly {
			if err := hydrateOCIOnly(interactive); err != nil {
				return err
			}
		} else {
			if err := hydrateCommon(interactive); err != nil {
				return err
			}
			if err := config.ValidateVault(ociFlags); err != nil {
				return err
			}
		}
		if err := applyProfileObjectName(profile, cmd.Flags().Changed("object-name"), &restoreObjectName); err != nil {
			return err
		}
		printBanner()
		if restoreObjectName == "" && !interactive {
			return fmt.Errorf("provide --object-name to restore or use interactive mode to choose a backup")
		}
		oci, err := ociclient.NewClient(ociFlags)
		if err != nil {
			return err
		}
		selectedExactObject := false
		if restoreObjectName == "" && interactive {
			selectCtx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
			restoreObjectName, err = selectBackupObject(selectCtx, oci)
			cancel()
			if err != nil {
				return err
			}
			selectedExactObject = true
		}
		validateCtx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
		result, err := validateRestore(validateCtx, oci, restoreObjectName, selectedExactObject)
		cancel()
		if err != nil {
			return err
		}
		if hasInvalidValidateUsers(result.Users) {
			return fmt.Errorf("validation failed: one or more users do not have a valid password in Vault")
		}
		if restoreValidateOnly {
			fmt.Println("Validation completed successfully. No restore operation was executed.")
			return nil
		}
		if interactive {
			confirmed, err := confirmRestore()
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Println("Restore canceled by user.")
				return nil
			}
		}
		ka, err := kafkaadmin.NewAdmin(kafkaFlags)
		if err != nil {
			return err
		}
		defer ka.Close()
		fmt.Println("Restoring SCRAM users and ACLs...")
		restoreCtx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
		defer cancel()
		if err := ka.Restore(restoreCtx, result.Backup, result.Passwords); err != nil {
			return err
		}
		fmt.Println("Restore completed successfully.")
		return nil
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restoreObjectName, "object-name", "", "Backup JSON file name. The configured backup prefix is applied automatically unless this already includes it")
	restoreCmd.Flags().BoolVar(&restoreValidateOnly, "validate", false, "Validate that all backup users have Vault passwords without executing restore")
	restoreCmd.Flags().Bool("interactive", true, "Prompt for missing required values and confirmation")
	rootCmd.AddCommand(restoreCmd)
}

func validateRestore(ctx context.Context, client *ociclient.Client, objectName string, exactObjectName bool) (*restoreValidationResult, error) {
	fmt.Println("Downloading backup from Object Storage...")
	var b *backup.File
	var err error
	if exactObjectName {
		b, err = client.DownloadBackupExact(ctx, objectName)
	} else {
		b, err = client.DownloadBackup(ctx, objectName)
	}
	if err != nil {
		return nil, err
	}
	fmt.Printf("Backup contains %d users and %d ACLs.\n", len(b.Users), len(b.ACLs))
	fmt.Println("Validating backup and OCI Vault secrets...")
	validations, passwords, err := client.ValidateAndLoadPasswords(ctx, b.Users)
	if err != nil {
		return nil, err
	}
	users := usersFromVaultValidations(validations)
	printValidateReport("restore", b, users)
	return &restoreValidationResult{Backup: b, Users: users, Passwords: passwords}, nil
}

func selectBackupObject(ctx context.Context, client *ociclient.Client) (string, error) {
	fmt.Println("Listing backup directories in Object Storage...")
	prefixes, err := client.ListBackupPrefixes(ctx)
	if err != nil {
		return "", err
	}
	if len(prefixes) == 0 {
		return "", fmt.Errorf("no JSON backups found in the bucket")
	}
	prefix, err := selectBackupPrefix(prefixes)
	if err != nil {
		return "", err
	}
	fmt.Printf("Listing backups in %s...\n", displayPrefix(prefix))
	backups, err := client.ListBackupsInPrefix(ctx, prefix)
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("no JSON backups found in %s", displayPrefix(prefix))
	}
	return selectBackupFile(backups)
}

func selectBackupPrefix(prefixes []string) (string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "> {{ . }}",
		Inactive: "  {{ . }}",
		Selected: "Selected directory: {{ . }}",
	}
	options := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		options = append(options, displayPrefix(prefix))
	}
	prompt := promptui.Select{
		Label:     "Choose the backup directory",
		Items:     options,
		Templates: templates,
		Size:      len(options),
	}
	index, _, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return prefixes[index], nil
}

func selectBackupFile(backups []ociclient.BackupObject) (string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "> {{ .Name }}  {{ .LastModified }}  {{ .Size }}",
		Inactive: "  {{ .Name }}  {{ .LastModified }}  {{ .Size }}",
		Selected: "Selected backup: {{ .Name }}",
	}
	type option struct {
		Name         string
		LastModified string
		Size         string
	}
	options := make([]option, 0, len(backups))
	for _, b := range backups {
		options = append(options, option{
			Name:         b.Name,
			LastModified: formatBackupTime(b.LastModified),
			Size:         formatBytes(b.Size),
		})
	}
	prompt := promptui.Select{
		Label:     "Choose the backup to restore",
		Items:     options,
		Templates: templates,
		Size:      len(options),
	}
	index, _, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return backups[index].Name, nil
}

func displayPrefix(prefix string) string {
	if prefix == "/" {
		return "(bucket root)"
	}
	return prefix + "/"
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func confirmRestore() (bool, error) {
	prompt := promptui.Prompt{
		Label:   "Apply restore to the target Kafka cluster? (Y/n)",
		Default: "Y",
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
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y", nil
}

func formatBackupTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
