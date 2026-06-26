package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"kguard/internal/backup"
	kafkaadmin "kguard/internal/kafka"
	ociclient "kguard/internal/oci"

	"github.com/spf13/cobra"
)

var backupObjectName string

var backupCmd = &cobra.Command{
	Use:           "backup",
	Short:         "Back up Kafka SCRAM users and ACLs to OCI Object Storage",
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		profile, err := selectedConfigProfile()
		if err != nil {
			return err
		}
		if err := hydrateCommon(interactive); err != nil {
			return err
		}
		if err := applyProfileObjectName(profile, cmd.Flags().Changed("object-name"), &backupObjectName); err != nil {
			return err
		}
		printBanner()
		ctx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
		defer cancel()
		if backupObjectName == "" {
			backupObjectName = fmt.Sprintf("kafka-acl-backup-%s.json", time.Now().UTC().Format("20060102T150405Z"))
		}
		fmt.Println("Connecting to Kafka and collecting users/ACLs...")
		ka, err := kafkaadmin.NewAdmin(kafkaFlags)
		if err != nil {
			return err
		}
		defer ka.Close()
		b, err := ka.Backup(ctx, kafkaFlags.BootstrapServers)
		if err != nil {
			return err
		}
		fmt.Printf("Found %d users and %d ACLs.\n", len(b.Users), len(b.ACLs))
		if err := validateBackupData(b); err != nil {
			return err
		}
		fmt.Println("Uploading backup to OCI...")
		oci, err := ociclient.NewClient(ociFlags)
		if err != nil {
			return err
		}
		if err := oci.UploadBackup(ctx, backupObjectName, b); err != nil {
			return err
		}
		fmt.Printf("Backup saved: bucket=%s object=%s\n", ociFlags.Bucket, backupObjectName)
		return nil
	},
}

func validateBackupData(b *backup.File) error {
	userValidations := validateBackupUsers(b.Users)
	printValidateReport("backup", b, userValidations)
	if hasInvalidValidateUsers(userValidations) {
		return fmt.Errorf("backup validation failed: one or more users are invalid")
	}
	if err := validateBackupACLs(b.ACLs); err != nil {
		return err
	}
	fmt.Println("Backup validation completed successfully.")
	return nil
}

func validateBackupUsers(users []backup.User) []validateUserRow {
	if len(users) == 0 {
		return []validateUserRow{{
			User:    "-",
			Warning: true,
			Message: "no SCRAM users found",
		}}
	}
	validations := make([]validateUserRow, 0, len(users))
	for _, user := range users {
		validation := validateUserRow{User: user.Name, Valid: true, Message: "SCRAM credentials found"}
		if strings.TrimSpace(user.Name) == "" {
			validation.User = "-"
			validation.Valid = false
			validation.Message = "user name is empty"
		} else if len(user.Credentials) == 0 {
			validation.Valid = false
			validation.Message = "SCRAM credentials not found"
		}
		validations = append(validations, validation)
	}
	sort.Slice(validations, func(i, j int) bool {
		return validations[i].User < validations[j].User
	})
	return validations
}

func validateBackupACLs(acls []backup.ACL) error {
	for i, acl := range acls {
		if strings.TrimSpace(acl.Principal) == "" {
			return fmt.Errorf("backup validation failed: ACL %d has an empty principal", i+1)
		}
		if strings.TrimSpace(acl.ResourceType) == "" {
			return fmt.Errorf("backup validation failed: ACL %d has an empty resource type", i+1)
		}
		if strings.TrimSpace(acl.ResourceName) == "" {
			return fmt.Errorf("backup validation failed: ACL %d has an empty resource name", i+1)
		}
		if strings.TrimSpace(acl.ResourcePatternType) == "" {
			return fmt.Errorf("backup validation failed: ACL %d has an empty resource pattern type", i+1)
		}
		if strings.TrimSpace(acl.Operation) == "" {
			return fmt.Errorf("backup validation failed: ACL %d has an empty operation", i+1)
		}
		if strings.TrimSpace(acl.PermissionType) == "" {
			return fmt.Errorf("backup validation failed: ACL %d has an empty permission type", i+1)
		}
	}
	return nil
}

func init() {
	backupCmd.Flags().StringVar(&backupObjectName, "object-name", "", "Backup JSON file name. The configured backup prefix is applied automatically unless this already includes it")
	backupCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	rootCmd.AddCommand(backupCmd)
}
