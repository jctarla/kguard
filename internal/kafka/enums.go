package kafka

import (
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kmsg"
)

func nowUTC() time.Time { return time.Now().UTC() }

func scramMechanismName(id int8) string {
	switch id {
	case 1:
		return "SCRAM-SHA-256"
	case 2:
		return "SCRAM-SHA-512"
	default:
		return "UNKNOWN"
	}
}

func scramMechanismID(v string) (int8, error) {
	switch strings.ToUpper(v) {
	case "SCRAM-SHA-256":
		return 1, nil
	case "SCRAM-SHA-512", "":
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid SCRAM mechanism: %s", v)
	}
}

func aclResourceType(v string) (kmsg.ACLResourceType, error) {
	switch strings.ToUpper(v) {
	case "TOPIC":
		return kmsg.ACLResourceTypeTopic, nil
	case "GROUP":
		return kmsg.ACLResourceTypeGroup, nil
	case "CLUSTER":
		return kmsg.ACLResourceTypeCluster, nil
	case "TRANSACTIONAL_ID":
		return kmsg.ACLResourceTypeTransactionalId, nil
	case "DELEGATION_TOKEN":
		return kmsg.ACLResourceTypeDelegationToken, nil
	default:
		return 0, fmt.Errorf("invalid resource_type: %s", v)
	}
}

func aclPatternType(v string) (kmsg.ACLResourcePatternType, error) {
	switch strings.ToUpper(v) {
	case "LITERAL":
		return kmsg.ACLResourcePatternTypeLiteral, nil
	case "PREFIXED":
		return kmsg.ACLResourcePatternTypePrefixed, nil
	default:
		return 0, fmt.Errorf("invalid resource_pattern_type: %s", v)
	}
}

func aclOperation(v string) (kmsg.ACLOperation, error) {
	switch strings.ToUpper(v) {
	case "READ":
		return kmsg.ACLOperationRead, nil
	case "WRITE":
		return kmsg.ACLOperationWrite, nil
	case "CREATE":
		return kmsg.ACLOperationCreate, nil
	case "DELETE":
		return kmsg.ACLOperationDelete, nil
	case "ALTER":
		return kmsg.ACLOperationAlter, nil
	case "DESCRIBE":
		return kmsg.ACLOperationDescribe, nil
	case "CLUSTER_ACTION":
		return kmsg.ACLOperationClusterAction, nil
	case "DESCRIBE_CONFIGS":
		return kmsg.ACLOperationDescribeConfigs, nil
	case "ALTER_CONFIGS":
		return kmsg.ACLOperationAlterConfigs, nil
	case "IDEMPOTENT_WRITE":
		return kmsg.ACLOperationIdempotentWrite, nil
	case "CREATE_TOKENS":
		return kmsg.ACLOperationCreateTokens, nil
	case "DESCRIBE_TOKENS":
		return kmsg.ACLOperationDescribeTokens, nil
	case "ALL":
		return kmsg.ACLOperationAll, nil
	default:
		return 0, fmt.Errorf("invalid operation: %s", v)
	}
}

func aclPermission(v string) (kmsg.ACLPermissionType, error) {
	switch strings.ToUpper(v) {
	case "ALLOW":
		return kmsg.ACLPermissionTypeAllow, nil
	case "DENY":
		return kmsg.ACLPermissionTypeDeny, nil
	default:
		return 0, fmt.Errorf("invalid permission_type: %s", v)
	}
}
