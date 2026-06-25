package config

import (
	"errors"
	"strings"
	"time"
)

type Kafka struct {
	BootstrapServers []string
	Username         string
	Password         string
	Timeout          time.Duration
}

type OCI struct {
	Namespace     string
	Bucket        string
	Prefix        string
	Region        string
	CompartmentID string
	VaultID       string
	Profile       string
	ConfigPath    string
}

func SplitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ValidateKafka(c Kafka) error {
	if len(c.BootstrapServers) == 0 {
		return errors.New("provide at least one Kafka bootstrap server")
	}
	if strings.TrimSpace(c.Username) == "" {
		return errors.New("provide the Kafka user for SCRAM-SHA-512 authentication")
	}
	if c.Password == "" {
		return errors.New("provide the Kafka password for SCRAM-SHA-512 authentication")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be greater than zero")
	}
	return nil
}

func ValidateOCI(c OCI) error {
	if strings.TrimSpace(c.Namespace) == "" {
		return errors.New("provide the OCI Object Storage namespace")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return errors.New("provide the OCI Object Storage bucket")
	}
	return nil
}
