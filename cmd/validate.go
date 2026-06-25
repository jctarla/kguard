package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"kguard/internal/backup"
	ociclient "kguard/internal/oci"
)

type validateUserRow struct {
	User    string
	Secret  string
	Valid   bool
	Warning bool
	Message string
}

func printValidateReport(mode string, b *backup.File, users []validateUserRow) {
	fmt.Println()
	fmt.Println("VALIDATE")
	if mode != "" {
		fmt.Printf("Mode: %s\n", mode)
	}
	fmt.Printf("Backup created at: %s\n", formatBackupTime(b.CreatedAt))
	fmt.Printf("Backup Kafka cluster: %s\n", joinOrDash(b.Cluster.BootstrapServers))
	fmt.Printf("Target Kafka cluster: %s\n", joinOrDash(kafkaFlags.BootstrapServers))
	fmt.Printf("Users: %d\n", len(b.Users))
	fmt.Printf("ACLs: %d\n", len(b.ACLs))
	fmt.Println()
	printValidateUsersTable(users)
	fmt.Println()
	printValidateACLsTable(b.ACLs)
	fmt.Println()
}

func printValidateUsersTable(users []validateUserRow) {
	if len(users) == 0 {
		users = []validateUserRow{{
			User:    "-",
			Secret:  "-",
			Warning: true,
			Message: "no SCRAM users found",
		}}
	}
	users = append([]validateUserRow(nil), users...)
	sort.Slice(users, func(i, j int) bool {
		return users[i].User < users[j].User
	})
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USER\tSECRET\tSTATUS\tMESSAGE")
	for _, user := range users {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", user.User, valueOrDash(user.Secret), statusText(user), user.Message)
	}
	_ = w.Flush()
}

func printValidateACLsTable(acls []backup.ACL) {
	if len(acls) == 0 {
		fmt.Println("No ACLs found.")
		return
	}
	acls = append([]backup.ACL(nil), acls...)
	sort.Slice(acls, func(i, j int) bool {
		left := acls[i].Principal + acls[i].ResourceType + acls[i].ResourceName + acls[i].Operation
		right := acls[j].Principal + acls[j].ResourceType + acls[j].ResourceName + acls[j].Operation
		return left < right
	})
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PRINCIPAL\tRESOURCE\tPATTERN\tOPERATION\tPERMISSION\tHOST")
	for _, acl := range acls {
		resource := acl.ResourceType + ":" + acl.ResourceName
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", acl.Principal, resource, acl.ResourcePatternType, acl.Operation, acl.PermissionType, acl.Host)
	}
	_ = w.Flush()
}

func usersFromVaultValidations(validations []ociclient.PasswordValidation) []validateUserRow {
	users := make([]validateUserRow, 0, len(validations))
	for _, validation := range validations {
		users = append(users, validateUserRow{
			User:    validation.User,
			Secret:  validation.Secret,
			Valid:   validation.Valid,
			Message: validation.Message,
		})
	}
	return users
}

func hasInvalidValidateUsers(users []validateUserRow) bool {
	for _, user := range users {
		if !user.Valid && !user.Warning {
			return true
		}
	}
	return false
}

func statusText(user validateUserRow) string {
	if user.Warning {
		return "WARN"
	}
	if user.Valid {
		return "OK"
	}
	return "FAIL"
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
