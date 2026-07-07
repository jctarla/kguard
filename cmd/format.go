package cmd

import (
	"fmt"
	"strings"

	"kguard/internal/backup"
)

func joinCredentials(credentials []backup.Credential) string {
	if len(credentials) == 0 {
		return "-"
	}
	values := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		values = append(values, fmt.Sprintf("%s/%d", credential.Mechanism, credential.Iterations))
	}
	return strings.Join(values, ",")
}
