package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	kafkaadmin "kguard/internal/kafka"
	ociclient "kguard/internal/oci"

	"github.com/spf13/cobra"
)

var userCreatePassword string
var userScramMechanism string
var userScramIterations int32

var userCmd = &cobra.Command{
	Use:           "user",
	Short:         "Manage Kafka SCRAM users and matching OCI Vault secrets",
	SilenceErrors: true,
}

var userCreateCmd = &cobra.Command{
	Use:           "create [name]",
	Short:         "Create a Kafka SCRAM user and store its password in OCI Vault",
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		if err := hydrateKafkaAndVaultCreateSecret(interactive); err != nil {
			return err
		}
		name, err := userNameFromArgsOrJSON(args)
		if err != nil {
			return err
		}
		password := userCreatePassword
		if password == "" && interactive {
			var err error
			password, err = ask("Kafka user password", "", true)
			if err != nil {
				return err
			}
		}
		if password == "" {
			return fmt.Errorf("provide --password or use interactive mode")
		}
		printBanner()
		ctx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
		defer cancel()
		oci, err := ociclient.NewClient(ociFlags)
		if err != nil {
			return err
		}
		exists, err := oci.SecretExists(ctx, name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("Vault secret %q already exists", name)
		}
		ka, err := kafkaadmin.NewAdmin(kafkaFlags)
		if err != nil {
			return err
		}
		defer ka.Close()
		fmt.Printf("Creating Kafka user %s...\n", name)
		if err := ka.CreateUser(ctx, name, password, userScramMechanism, userScramIterations); err != nil {
			return err
		}
		fmt.Printf("Creating Vault secret %s...\n", name)
		if err := oci.CreatePasswordSecret(ctx, name, password); err != nil {
			_ = ka.DeleteUser(ctx, name, userScramMechanism)
			return err
		}
		fmt.Printf("User created: %s secret=%s\n", name, name)
		return nil
	},
}

var userDeleteCmd = &cobra.Command{
	Use:           "delete [name]",
	Short:         "Delete a Kafka SCRAM user and schedule deletion of its OCI Vault secret",
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		if err := hydrateKafkaAndVault(interactive); err != nil {
			return err
		}
		name, err := userNameFromArgsOrJSON(args)
		if err != nil {
			return err
		}
		printBanner()
		ctx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
		defer cancel()
		oci, err := ociclient.NewClient(ociFlags)
		if err != nil {
			return err
		}
		ka, err := kafkaadmin.NewAdmin(kafkaFlags)
		if err != nil {
			return err
		}
		defer ka.Close()
		fmt.Printf("Scheduling Vault secret deletion %s...\n", name)
		if err := oci.DeletePasswordSecret(ctx, name); err != nil {
			return err
		}
		fmt.Printf("Deleting Kafka user %s...\n", name)
		if err := ka.DeleteUser(ctx, name, userScramMechanism); err != nil {
			return err
		}
		fmt.Printf("User deleted: %s secret=%s\n", name, name)
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:           "list",
	Short:         "List Kafka SCRAM users",
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		if err := hydrateKafkaOnly(interactive); err != nil {
			return err
		}
		printBanner()
		ctx, cancel := context.WithTimeout(context.Background(), kafkaFlags.Timeout)
		defer cancel()
		ka, err := kafkaadmin.NewAdmin(kafkaFlags)
		if err != nil {
			return err
		}
		defer ka.Close()
		users, err := ka.ListUsers(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "USER\tMECHANISMS")
		for _, user := range users {
			fmt.Fprintf(w, "%s\t%s\n", user.Name, joinCredentials(user.Credentials))
		}
		return w.Flush()
	},
}

func init() {
	userCreateCmd.Flags().StringVar(&userCreatePassword, "password", "", "Password for the Kafka SCRAM user. If omitted in interactive mode, kguard prompts for it")
	userCreateCmd.Flags().StringVar(&userScramMechanism, "mechanism", "SCRAM-SHA-512", "SCRAM mechanism: SCRAM-SHA-256 or SCRAM-SHA-512")
	userCreateCmd.Flags().Int32Var(&userScramIterations, "iterations", 4096, "SCRAM iteration count")
	userCreateCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	userDeleteCmd.Flags().StringVar(&userScramMechanism, "mechanism", "SCRAM-SHA-512", "SCRAM mechanism to delete: SCRAM-SHA-256 or SCRAM-SHA-512")
	userDeleteCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	userListCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	userCmd.AddCommand(userCreateCmd, userDeleteCmd, userListCmd)
	rootCmd.AddCommand(userCmd)
}

func userNameFromArgsOrJSON(args []string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	for _, key := range []string{"username", "user", "name"} {
		if value := stringFromJSON(key); value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("provide the user name as an argument or set username in --from-json")
}
