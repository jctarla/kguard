package kafka

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/crypto/pbkdf2"

	"kguard/internal/backup"
	"kguard/internal/config"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

type Admin struct {
	client *kgo.Client
}

const ignoredUserPrefix = "super-user"

func NewAdmin(c config.Kafka) (*Admin, error) {
	if err := config.ValidateKafka(c); err != nil {
		return nil, err
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(c.BootstrapServers...),
		kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}),
		kgo.SASL(scram.Auth{User: c.Username, Pass: c.Password}.AsSha512Mechanism()),
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &Admin{client: cl}, nil
}

func (a *Admin) Close() {
	a.client.Close()
}

func (a *Admin) Backup(ctx context.Context, servers []string) (*backup.File, error) {
	users, err := a.describeUsers(ctx)
	if err != nil {
		return nil, err
	}
	acls, err := a.describeACLs(ctx)
	if err != nil {
		return nil, err
	}
	return &backup.File{
		Version:   backup.CurrentVersion,
		CreatedAt: nowUTC(),
		Cluster:   backup.Cluster{BootstrapServers: servers},
		Users:     users,
		ACLs:      acls,
	}, nil
}

func (a *Admin) Restore(ctx context.Context, b *backup.File, passwords map[string]string) error {
	if b.Version == "" {
		return fmt.Errorf("backup has no version")
	}
	if err := a.restoreUsers(ctx, b.Users, passwords); err != nil {
		return err
	}
	return a.restoreACLs(ctx, b.ACLs)
}

func (a *Admin) describeUsers(ctx context.Context) ([]backup.User, error) {
	req := kmsg.NewDescribeUserSCRAMCredentialsRequest()
	resp, err := req.RequestWith(ctx, a.client)
	if err != nil {
		return nil, fmt.Errorf("list SCRAM credentials: %w", err)
	}
	var users []backup.User
	for _, r := range resp.Results {
		if r.ErrorCode != 0 {
			return nil, fmt.Errorf("list SCRAM user %q: kafka error code %d: %s", r.User, r.ErrorCode, msg(r.ErrorMessage))
		}
		if shouldIgnoreUser(r.User) {
			continue
		}
		u := backup.User{Name: r.User}
		for _, ci := range r.CredentialInfos {
			u.Credentials = append(u.Credentials, backup.Credential{
				Mechanism:  scramMechanismName(ci.Mechanism),
				Iterations: ci.Iterations,
			})
		}
		users = append(users, u)
	}
	return users, nil
}

func (a *Admin) describeACLs(ctx context.Context) ([]backup.ACL, error) {
	req := kmsg.NewDescribeACLsRequest()
	req.ResourceType = kmsg.ACLResourceTypeAny
	req.ResourcePatternType = kmsg.ACLResourcePatternTypeAny
	req.Operation = kmsg.ACLOperationAny
	req.PermissionType = kmsg.ACLPermissionTypeAny
	resp, err := req.RequestWith(ctx, a.client)
	if err != nil {
		return nil, fmt.Errorf("list ACLs: %w", err)
	}
	if resp.ErrorCode != 0 {
		return nil, fmt.Errorf("list ACLs: kafka error code %d: %s", resp.ErrorCode, msg(resp.ErrorMessage))
	}
	var acls []backup.ACL
	for _, r := range resp.Resources {
		for _, acl := range r.ACLs {
			if shouldIgnorePrincipal(acl.Principal) {
				continue
			}
			acls = append(acls, backup.ACL{
				ResourceType:        r.ResourceType.String(),
				ResourceName:        r.ResourceName,
				ResourcePatternType: r.ResourcePatternType.String(),
				Principal:           acl.Principal,
				Host:                acl.Host,
				Operation:           acl.Operation.String(),
				PermissionType:      acl.PermissionType.String(),
			})
		}
	}
	return acls, nil
}

func (a *Admin) restoreUsers(ctx context.Context, users []backup.User, passwords map[string]string) error {
	req := kmsg.NewAlterUserSCRAMCredentialsRequest()
	for _, u := range users {
		pass := passwords[u.Name]
		if pass == "" {
			return fmt.Errorf("password not found in Vault for user %q", u.Name)
		}
		for _, c := range u.Credentials {
			mech, err := scramMechanismID(c.Mechanism)
			if err != nil {
				return err
			}
			iterations := c.Iterations
			if iterations == 0 {
				iterations = 4096
			}
			salt := make([]byte, 32)
			if _, err := rand.Read(salt); err != nil {
				return fmt.Errorf("generate SCRAM salt for %q: %w", u.Name, err)
			}
			req.Upsertions = append(req.Upsertions, kmsg.AlterUserSCRAMCredentialsRequestUpsertion{
				Name:           u.Name,
				Mechanism:      mech,
				Iterations:     iterations,
				Salt:           salt,
				SaltedPassword: saltedPassword(c.Mechanism, pass, salt, iterations),
			})
		}
	}
	if len(req.Upsertions) == 0 {
		return nil
	}
	resp, err := req.RequestWith(ctx, a.client)
	if err != nil {
		return fmt.Errorf("restore SCRAM users: %w", err)
	}
	for _, r := range resp.Results {
		if r.ErrorCode != 0 {
			return fmt.Errorf("restore SCRAM user %q: kafka error code %d: %s", r.User, r.ErrorCode, msg(r.ErrorMessage))
		}
	}
	return nil
}

func (a *Admin) restoreACLs(ctx context.Context, acls []backup.ACL) error {
	if err := a.deleteExistingACLsForBackupPrincipals(ctx, acls); err != nil {
		return err
	}
	req := kmsg.NewCreateACLsRequest()
	for _, acl := range acls {
		rt, err := aclResourceType(acl.ResourceType)
		if err != nil {
			return err
		}
		pt, err := aclPatternType(acl.ResourcePatternType)
		if err != nil {
			return err
		}
		op, err := aclOperation(acl.Operation)
		if err != nil {
			return err
		}
		perm, err := aclPermission(acl.PermissionType)
		if err != nil {
			return err
		}
		req.Creations = append(req.Creations, kmsg.CreateACLsRequestCreation{
			ResourceType:        rt,
			ResourceName:        acl.ResourceName,
			ResourcePatternType: pt,
			Principal:           acl.Principal,
			Host:                acl.Host,
			Operation:           op,
			PermissionType:      perm,
		})
	}
	if len(req.Creations) == 0 {
		return nil
	}
	resp, err := req.RequestWith(ctx, a.client)
	if err != nil {
		return fmt.Errorf("restore ACLs: %w", err)
	}
	for i, r := range resp.Results {
		if r.ErrorCode != 0 {
			return fmt.Errorf("restore ACL %d: kafka error code %d: %s", i+1, r.ErrorCode, msg(r.ErrorMessage))
		}
	}
	return nil
}

func (a *Admin) deleteExistingACLsForBackupPrincipals(ctx context.Context, acls []backup.ACL) error {
	principals := backupACLPrincipals(acls)
	if len(principals) == 0 {
		return nil
	}
	req := kmsg.NewDeleteACLsRequest()
	for _, principal := range principals {
		principal := principal
		req.Filters = append(req.Filters, kmsg.DeleteACLsRequestFilter{
			ResourceType:        kmsg.ACLResourceTypeAny,
			ResourcePatternType: kmsg.ACLResourcePatternTypeAny,
			Principal:           &principal,
			Operation:           kmsg.ACLOperationAny,
			PermissionType:      kmsg.ACLPermissionTypeAny,
		})
	}
	resp, err := req.RequestWith(ctx, a.client)
	if err != nil {
		return fmt.Errorf("delete existing ACLs before restore: %w", err)
	}
	for i, r := range resp.Results {
		if r.ErrorCode != 0 {
			return fmt.Errorf("delete existing ACLs for principal %q: kafka error code %d: %s", principals[i], r.ErrorCode, msg(r.ErrorMessage))
		}
	}
	return nil
}

func backupACLPrincipals(acls []backup.ACL) []string {
	seen := make(map[string]struct{})
	for _, acl := range acls {
		if acl.Principal == "" {
			continue
		}
		seen[acl.Principal] = struct{}{}
	}
	principals := make([]string, 0, len(seen))
	for principal := range seen {
		principals = append(principals, principal)
	}
	sort.Strings(principals)
	return principals
}

func saltedPassword(mechanism, password string, salt []byte, iterations int32) []byte {
	if strings.EqualFold(mechanism, "SCRAM-SHA-512") {
		return pbkdf2.Key([]byte(password), salt, int(iterations), sha512.Size, sha512.New)
	}
	return pbkdf2.Key([]byte(password), salt, int(iterations), sha256.Size, sha256.New)
}

func msg(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func shouldIgnoreUser(name string) bool {
	return strings.HasPrefix(name, ignoredUserPrefix)
}

func shouldIgnorePrincipal(principal string) bool {
	return shouldIgnoreUser(strings.TrimPrefix(principal, "User:"))
}
