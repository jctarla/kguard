package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"kguard/internal/backup"
	kafkaadmin "kguard/internal/kafka"

	"github.com/spf13/cobra"
)

type aclCommandFlags struct {
	Topics              []string
	Groups              []string
	TransactionalIDs    []string
	DelegationTokens    []string
	UserPrincipals      []string
	ResourcePatternType string
	AllowPrincipals     []string
	DenyPrincipals      []string
	Principals          []string
	AllowHosts          []string
	DenyHosts           []string
	Operations          []string
	Producer            bool
	Consumer            bool
	Idempotent          bool
	Cluster             bool
	Force               bool
}

var aclFlags aclCommandFlags

var aclCmd = &cobra.Command{
	Use:           "acl",
	Short:         "Manage Kafka ACLs",
	SilenceErrors: true,
}

var aclCreateCmd = &cobra.Command{
	Use:           "create",
	Aliases:       []string{"add"},
	Short:         "Create Kafka ACLs",
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		if err := hydrateKafkaOnly(interactive); err != nil {
			return err
		}
		acls, err := buildACLsForWrite(aclFlags)
		if err != nil {
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
		for _, acl := range acls {
			if err := ka.CreateACL(ctx, acl); err != nil {
				return err
			}
		}
		fmt.Printf("ACLs created: %d\n", len(acls))
		return nil
	},
}

var aclDeleteCmd = &cobra.Command{
	Use:           "delete",
	Aliases:       []string{"remove"},
	Short:         "Delete Kafka ACLs matching the provided fields",
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		interactive, _ := cmd.Flags().GetBool("interactive")
		if err := hydrateKafkaOnly(interactive); err != nil {
			return err
		}
		acls, err := buildACLsForWrite(aclFlags)
		if err != nil {
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
		for _, acl := range acls {
			if err := ka.DeleteACL(ctx, acl); err != nil {
				return err
			}
		}
		fmt.Printf("ACLs deleted: %d\n", len(acls))
		return nil
	},
}

var aclListCmd = &cobra.Command{
	Use:           "list",
	Short:         "List Kafka ACLs",
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
		acls, err := ka.ListACLs(ctx)
		if err != nil {
			return err
		}
		acls = filterListedACLs(acls, aclFlags)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PRINCIPAL\tHOST\tRESOURCE\tPATTERN\tOPERATION\tPERMISSION")
		for _, acl := range acls {
			fmt.Fprintf(w, "%s\t%s\t%s:%s\t%s\t%s\t%s\n", acl.Principal, acl.Host, acl.ResourceType, acl.ResourceName, acl.ResourcePatternType, acl.Operation, acl.PermissionType)
		}
		return w.Flush()
	},
}

func init() {
	addACLFlags(aclCreateCmd, true)
	addACLFlags(aclDeleteCmd, true)
	addACLFlags(aclListCmd, false)
	aclCreateCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	aclDeleteCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	aclListCmd.Flags().Bool("interactive", true, "Prompt for missing required values")
	aclCmd.AddCommand(aclCreateCmd, aclDeleteCmd, aclListCmd)
	rootCmd.AddCommand(aclCmd)
}

func addACLFlags(cmd *cobra.Command, write bool) {
	cmd.Flags().StringArrayVar(&aclFlags.Topics, "topic", nil, "Topic resource pattern. Can be specified multiple times")
	cmd.Flags().StringArrayVar(&aclFlags.Groups, "group", nil, "Consumer group resource pattern. Can be specified multiple times")
	cmd.Flags().BoolVar(&aclFlags.Cluster, "cluster", false, "Use the singular cluster resource")
	cmd.Flags().StringArrayVar(&aclFlags.TransactionalIDs, "transactional-id", nil, "TransactionalId resource pattern. Can be specified multiple times")
	cmd.Flags().StringArrayVar(&aclFlags.DelegationTokens, "delegation-token", nil, "DelegationToken resource pattern. Can be specified multiple times")
	cmd.Flags().StringArrayVar(&aclFlags.UserPrincipals, "user-principal", nil, "User resource pattern. Can be specified multiple times")
	cmd.Flags().StringVar(&aclFlags.ResourcePatternType, "resource-pattern-type", "literal", "Resource pattern type: literal or prefixed")
	if write {
		cmd.Flags().StringArrayVar(&aclFlags.AllowPrincipals, "allow-principal", nil, "Principal to allow. Example: User:app-user. Can be specified multiple times")
		cmd.Flags().StringArrayVar(&aclFlags.DenyPrincipals, "deny-principal", nil, "Principal to deny. Example: User:app-user. Can be specified multiple times")
		cmd.Flags().StringArrayVar(&aclFlags.AllowHosts, "allow-host", nil, "Host for allowed principals. Defaults to * when --allow-principal is used")
		cmd.Flags().StringArrayVar(&aclFlags.DenyHosts, "deny-host", nil, "Host for denied principals. Defaults to * when --deny-principal is used")
		cmd.Flags().StringArrayVar(&aclFlags.Operations, "operation", nil, "ACL operation. Can be specified multiple times. Default: All")
		cmd.Flags().BoolVar(&aclFlags.Producer, "producer", false, "Convenience option for producer ACLs on topic resources")
		cmd.Flags().BoolVar(&aclFlags.Consumer, "consumer", false, "Convenience option for consumer ACLs on topic and group resources")
		cmd.Flags().BoolVar(&aclFlags.Idempotent, "idempotent", false, "Add idempotent producer ACL on the cluster resource")
		cmd.Flags().BoolVar(&aclFlags.Force, "force", false, "Assume yes to prompts")
	} else {
		cmd.Flags().StringArrayVar(&aclFlags.Principals, "principal", nil, "Principal filter for list. Example: User:app-user")
	}
}

func buildACLsForWrite(flags aclCommandFlags) ([]backup.ACL, error) {
	if flags.Producer && len(flags.Topics) == 0 {
		return nil, fmt.Errorf("--producer requires --topic")
	}
	if flags.Consumer && (len(flags.Topics) == 0 || len(flags.Groups) == 0) {
		return nil, fmt.Errorf("--consumer requires --topic and --group")
	}
	resources := aclResources(flags)
	if len(resources) == 0 {
		return nil, fmt.Errorf("provide one resource flag: --topic, --group, --cluster, --transactional-id, --delegation-token, or --user-principal")
	}
	var entries []backup.ACL
	if len(flags.AllowPrincipals) > 0 {
		hosts := withDefault(flags.AllowHosts, "*")
		entries = append(entries, expandACLsForMode(resources, flags.AllowPrincipals, hosts, flags, "ALLOW")...)
	}
	if len(flags.DenyPrincipals) > 0 {
		hosts := withDefault(flags.DenyHosts, "*")
		entries = append(entries, expandACLsForMode(resources, flags.DenyPrincipals, hosts, flags, "DENY")...)
	}
	if len(entries) == 0 {
		if flags.Producer || flags.Consumer {
			return nil, fmt.Errorf("--producer requires --topic; --consumer requires --topic and --group")
		}
		return nil, fmt.Errorf("provide --allow-principal or --deny-principal")
	}
	if flags.Idempotent {
		for _, principal := range flags.AllowPrincipals {
			entries = append(entries, backup.ACL{
				ResourceType:        "CLUSTER",
				ResourceName:        "kafka-cluster",
				ResourcePatternType: "LITERAL",
				Principal:           principal,
				Host:                "*",
				Operation:           "IDEMPOTENT_WRITE",
				PermissionType:      "ALLOW",
			})
		}
	}
	return entries, nil
}

func aclResources(flags aclCommandFlags) []backup.ACL {
	pattern := strings.ToUpper(flags.ResourcePatternType)
	var resources []backup.ACL
	for _, topic := range flags.Topics {
		resources = append(resources, backup.ACL{ResourceType: "TOPIC", ResourceName: topic, ResourcePatternType: pattern})
	}
	for _, group := range flags.Groups {
		resources = append(resources, backup.ACL{ResourceType: "GROUP", ResourceName: group, ResourcePatternType: pattern})
	}
	if flags.Cluster {
		resources = append(resources, backup.ACL{ResourceType: "CLUSTER", ResourceName: "kafka-cluster", ResourcePatternType: "LITERAL"})
	}
	for _, transactionalID := range flags.TransactionalIDs {
		resources = append(resources, backup.ACL{ResourceType: "TRANSACTIONAL_ID", ResourceName: transactionalID, ResourcePatternType: pattern})
	}
	for _, token := range flags.DelegationTokens {
		resources = append(resources, backup.ACL{ResourceType: "DELEGATION_TOKEN", ResourceName: token, ResourcePatternType: pattern})
	}
	for _, user := range flags.UserPrincipals {
		resources = append(resources, backup.ACL{ResourceType: "USER", ResourceName: user, ResourcePatternType: pattern})
	}
	return resources
}

func explicitOperations(flags aclCommandFlags) []string {
	if len(flags.Operations) > 0 {
		return flags.Operations
	}
	return []string{"ALL"}
}

func expandACLsForMode(resources []backup.ACL, principals, hosts []string, flags aclCommandFlags, permission string) []backup.ACL {
	if !flags.Producer && !flags.Consumer {
		return expandACLs(resources, principals, hosts, explicitOperations(flags), permission)
	}
	var acls []backup.ACL
	if flags.Producer {
		acls = append(acls, expandACLs(resourcesByType(resources, "TOPIC"), principals, hosts, []string{"WRITE", "DESCRIBE", "CREATE"}, permission)...)
	}
	if flags.Consumer {
		acls = append(acls, expandACLs(resourcesByType(resources, "TOPIC"), principals, hosts, []string{"READ", "DESCRIBE"}, permission)...)
		acls = append(acls, expandACLs(resourcesByType(resources, "GROUP"), principals, hosts, []string{"READ"}, permission)...)
	}
	return acls
}

func resourcesByType(resources []backup.ACL, resourceType string) []backup.ACL {
	var out []backup.ACL
	for _, resource := range resources {
		if strings.EqualFold(resource.ResourceType, resourceType) {
			out = append(out, resource)
		}
	}
	return out
}

func expandACLs(resources []backup.ACL, principals, hosts, operations []string, permission string) []backup.ACL {
	var acls []backup.ACL
	for _, resource := range resources {
		for _, principal := range principals {
			for _, host := range hosts {
				for _, operation := range operations {
					acl := resource
					acl.Principal = principal
					acl.Host = host
					acl.Operation = operation
					acl.PermissionType = permission
					acls = append(acls, acl)
				}
			}
		}
	}
	return acls
}

func withDefault(values []string, def string) []string {
	if len(values) == 0 {
		return []string{def}
	}
	return values
}

func filterListedACLs(acls []backup.ACL, flags aclCommandFlags) []backup.ACL {
	resources := aclResources(flags)
	principals := mapFromSlice(flags.Principals)
	if len(resources) == 0 && len(principals) == 0 {
		return acls
	}
	var out []backup.ACL
	for _, acl := range acls {
		if len(principals) > 0 {
			if _, ok := principals[acl.Principal]; !ok {
				continue
			}
		}
		if len(resources) > 0 && !matchesAnyResource(acl, resources) {
			continue
		}
		out = append(out, acl)
	}
	return out
}

func matchesAnyResource(acl backup.ACL, resources []backup.ACL) bool {
	for _, resource := range resources {
		if strings.EqualFold(acl.ResourceType, resource.ResourceType) && acl.ResourceName == resource.ResourceName {
			return true
		}
	}
	return false
}

func mapFromSlice(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
