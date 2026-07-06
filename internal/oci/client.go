package oci

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kguard/internal/backup"
	"kguard/internal/config"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/oracle/oci-go-sdk/v65/vault"
)

type Client struct {
	object objectstorage.ObjectStorageClient
	vault  vault.VaultsClient
	secret secrets.SecretsClient
	cfg    config.OCI
}

type PasswordValidation struct {
	User    string
	Secret  string
	Valid   bool
	Message string
}

type BackupObject struct {
	Name         string
	Size         int64
	LastModified time.Time
}

func NewClient(cfg config.OCI) (*Client, error) {
	if err := config.ValidateOCI(cfg); err != nil {
		return nil, err
	}
	provider, err := provider(cfg)
	if err != nil {
		return nil, err
	}
	objectClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("create Object Storage client: %w", err)
	}
	vaultClient, err := vault.NewVaultsClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("create Vault client: %w", err)
	}
	secretClient, err := secrets.NewSecretsClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("create Secrets client: %w", err)
	}
	return &Client{object: objectClient, vault: vaultClient, secret: secretClient, cfg: cfg}, nil
}

func (c *Client) UploadBackup(ctx context.Context, name string, b *backup.File) error {
	body, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar backup: %w", err)
	}
	name = c.objectName(name)
	_, err = c.object.PutObject(ctx, objectstorage.PutObjectRequest{
		NamespaceName: common.String(c.cfg.Namespace),
		BucketName:    common.String(c.cfg.Bucket),
		ObjectName:    common.String(name),
		ContentType:   common.String("application/json"),
		PutObjectBody: io.NopCloser(bytes.NewReader(body)),
		ContentLength: common.Int64(int64(len(body))),
		OpcMeta:       map[string]string{"created-by": "kguard"},
	})
	if err != nil {
		return fmt.Errorf("upload backup to Object Storage: %w", err)
	}
	return nil
}

func (c *Client) DownloadBackup(ctx context.Context, name string) (*backup.File, error) {
	name = c.objectName(name)
	return c.downloadBackupObject(ctx, name)
}

func (c *Client) DownloadBackupExact(ctx context.Context, name string) (*backup.File, error) {
	name = strings.TrimLeft(strings.TrimSpace(name), "/")
	return c.downloadBackupObject(ctx, name)
}

func (c *Client) downloadBackupObject(ctx context.Context, name string) (*backup.File, error) {
	resp, err := c.object.GetObject(ctx, objectstorage.GetObjectRequest{
		NamespaceName: common.String(c.cfg.Namespace),
		BucketName:    common.String(c.cfg.Bucket),
		ObjectName:    common.String(name),
	})
	if err != nil {
		return nil, fmt.Errorf("download backup from Object Storage: %w", err)
	}
	defer resp.Content.Close()
	var b backup.File
	if err := json.NewDecoder(resp.Content).Decode(&b); err != nil {
		return nil, fmt.Errorf("decode backup JSON: %w", err)
	}
	return &b, nil
}

func (c *Client) ListBackupPrefixes(ctx context.Context) ([]string, error) {
	seen := map[string]struct{}{}
	var start *string
	for {
		resp, err := c.object.ListObjects(ctx, objectstorage.ListObjectsRequest{
			NamespaceName: common.String(c.cfg.Namespace),
			BucketName:    common.String(c.cfg.Bucket),
			Start:         start,
			Limit:         common.Int(100),
			Fields:        common.String("name"),
		})
		if err != nil {
			return nil, fmt.Errorf("list backup prefixes in Object Storage: %w", err)
		}
		for _, obj := range resp.Objects {
			if obj.Name == nil || !strings.HasSuffix(strings.ToLower(*obj.Name), ".json") {
				continue
			}
			prefix := objectPrefix(*obj.Name)
			seen[prefix] = struct{}{}
		}
		if resp.NextStartWith == nil {
			break
		}
		start = resp.NextStartWith
	}
	prefixes := make([]string, 0, len(seen))
	for prefix := range seen {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes, nil
}

func (c *Client) ListBackupsInPrefix(ctx context.Context, prefix string) ([]BackupObject, error) {
	selectedPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	listPrefix := selectedPrefix
	if listPrefix != "" {
		listPrefix += "/"
	}
	var objects []BackupObject
	var start *string
	for {
		resp, err := c.object.ListObjects(ctx, objectstorage.ListObjectsRequest{
			NamespaceName: common.String(c.cfg.Namespace),
			BucketName:    common.String(c.cfg.Bucket),
			Prefix:        common.String(listPrefix),
			Start:         start,
			Limit:         common.Int(100),
			Fields:        common.String("name,size,timeCreated,timeModified"),
		})
		if err != nil {
			return nil, fmt.Errorf("list backups in Object Storage: %w", err)
		}
		for _, obj := range resp.Objects {
			if obj.Name == nil || !strings.HasSuffix(strings.ToLower(*obj.Name), ".json") {
				continue
			}
			if selectedPrefix == "" && objectPrefix(*obj.Name) != "/" {
				continue
			}
			objects = append(objects, BackupObject{
				Name:         *obj.Name,
				Size:         int64Value(obj.Size),
				LastModified: objectTime(obj),
			})
		}
		if resp.NextStartWith == nil {
			break
		}
		start = resp.NextStartWith
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})
	return objects, nil
}

func objectPrefix(name string) string {
	name = strings.TrimLeft(strings.TrimSpace(name), "/")
	dir, _, ok := strings.Cut(name, "/")
	if !ok {
		return "/"
	}
	return dir
}

func (c *Client) objectName(name string) string {
	name = strings.TrimLeft(strings.TrimSpace(name), "/")
	prefix := strings.Trim(strings.TrimSpace(c.cfg.Prefix), "/")
	if prefix == "" || strings.HasPrefix(name, prefix+"/") {
		return name
	}
	return prefix + "/" + name
}

func objectTime(obj objectstorage.ObjectSummary) time.Time {
	if obj.TimeModified != nil {
		return obj.TimeModified.Time
	}
	if obj.TimeCreated != nil {
		return obj.TimeCreated.Time
	}
	return time.Time{}
}

func int64Value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func (c *Client) LoadPasswords(ctx context.Context, users []backup.User) (map[string]string, error) {
	validations, passwords, err := c.validatePasswords(ctx, users, true)
	if err != nil {
		return nil, err
	}
	for _, v := range validations {
		if !v.Valid {
			return nil, fmt.Errorf("%s", v.Message)
		}
	}
	return passwords, nil
}

func (c *Client) ValidatePasswords(ctx context.Context, users []backup.User) ([]PasswordValidation, error) {
	validations, _, err := c.validatePasswords(ctx, users, false)
	return validations, err
}

func (c *Client) ValidateAndLoadPasswords(ctx context.Context, users []backup.User) ([]PasswordValidation, map[string]string, error) {
	return c.validatePasswords(ctx, users, true)
}

func (c *Client) validatePasswords(ctx context.Context, users []backup.User, keepPasswords bool) ([]PasswordValidation, map[string]string, error) {
	if strings.TrimSpace(c.cfg.VaultID) == "" {
		return nil, nil, fmt.Errorf("provide the Vault OCID to locate password secrets")
	}
	if strings.TrimSpace(c.cfg.CompartmentID) == "" {
		return nil, nil, fmt.Errorf("provide the compartment OCID to list Vault secrets")
	}
	secretsByName, err := c.listSecrets(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make(map[string]string, len(users))
	validations := make([]PasswordValidation, 0, len(users))
	for _, u := range users {
		validation := PasswordValidation{User: u.Name, Secret: u.Name}
		id := secretsByName[u.Name]
		if id == "" {
			validation.Message = fmt.Sprintf("secret %q was not found in the configured Vault", u.Name)
			validations = append(validations, validation)
			continue
		}
		value, err := c.getSecret(ctx, id)
		if err != nil {
			validation.Message = fmt.Sprintf("ler secret %q: %v", u.Name, err)
			validations = append(validations, validation)
			continue
		}
		if strings.TrimSpace(value) == "" {
			validation.Message = fmt.Sprintf("secret %q is empty", u.Name)
			validations = append(validations, validation)
			continue
		}
		validation.Valid = true
		validation.Message = "user password found on OCI Vault"
		validations = append(validations, validation)
		if keepPasswords {
			out[u.Name] = value
		}
	}
	return validations, out, nil
}

func (c *Client) listSecrets(ctx context.Context) (map[string]string, error) {
	result := map[string]string{}
	var page *string
	for {
		resp, err := c.vault.ListSecrets(ctx, vault.ListSecretsRequest{
			CompartmentId: common.String(c.cfg.CompartmentID),
			VaultId:       common.String(c.cfg.VaultID),
			Page:          page,
			Limit:         common.Int(100),
		})
		if err != nil {
			return nil, fmt.Errorf("list Vault secrets: %w", err)
		}
		for _, s := range resp.Items {
			if s.SecretName != nil && s.Id != nil {
				result[*s.SecretName] = *s.Id
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return result, nil
}

func (c *Client) getSecret(ctx context.Context, id string) (string, error) {
	resp, err := c.secret.GetSecretBundle(ctx, secrets.GetSecretBundleRequest{
		SecretId: common.String(id),
		Stage:    secrets.GetSecretBundleStageCurrent,
	})
	if err != nil {
		return "", err
	}
	content, ok := resp.SecretBundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok || content.Content == nil {
		return "", fmt.Errorf("secret content is not BASE64")
	}
	decoded, err := base64.StdEncoding.DecodeString(*content.Content)
	if err != nil {
		return "", fmt.Errorf("decode base64 content: %w", err)
	}
	return string(decoded), nil
}

func provider(cfg config.OCI) (common.ConfigurationProvider, error) {
	authMode := strings.ToUpper(strings.TrimSpace(cfg.AuthMode))
	if authMode == "" {
		authMode = "OCI_CONFIG"
	}
	switch authMode {
	case "OCI_CONFIG":
		return ociConfigProvider(cfg)
	case "INSTANCE_PRINCIPAL":
		p, err := auth.InstancePrincipalConfigurationProvider()
		if err != nil {
			return nil, fmt.Errorf("create Instance Principal OCI provider: %w", err)
		}
		return withRegion(p, cfg.Region), nil
	case "CLOUD_SHELL":
		return cloudShellProvider(cfg)
	default:
		return nil, fmt.Errorf("invalid OCI auth mode %q: use OCI_CONFIG, INSTANCE_PRINCIPAL, or CLOUD_SHELL", cfg.AuthMode)
	}
}

func ociConfigProvider(cfg config.OCI) (common.ConfigurationProvider, error) {
	configPath := cfg.ConfigPath
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".oci", "config")
	}
	profile := cfg.Profile
	if profile == "" {
		profile = "DEFAULT"
	}
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("OCI config file %q is not available: %w", configPath, err)
	}
	p, err := common.ConfigurationProviderFromFileWithProfile(configPath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("load OCI config profile %q from %q: %w", profile, configPath, err)
	}
	if ok, err := common.IsConfigurationProviderValid(p); !ok {
		if err != nil {
			return nil, fmt.Errorf("validate OCI config profile %q from %q: %w", profile, configPath, err)
		}
		return nil, fmt.Errorf("OCI config profile %q from %q is invalid", profile, configPath)
	}
	genericProvider, err := auth.GetGenericConfigurationProvider(p)
	if err != nil {
		return nil, fmt.Errorf("create OCI config provider: %w", err)
	}
	return withRegion(genericProvider, cfg.Region), nil
}

func cloudShellProvider(cfg config.OCI) (common.ConfigurationProvider, error) {
	p := common.DefaultConfigProvider()
	authConfig, err := p.AuthType()
	if err != nil {
		return nil, fmt.Errorf("load OCI Cloud Shell auth config: %w", err)
	}
	if authConfig.AuthType != common.InstancePrincipalDelegationToken || authConfig.OboToken == nil || strings.TrimSpace(*authConfig.OboToken) == "" {
		return nil, fmt.Errorf("CLOUD_SHELL auth requires an OCI Cloud Shell config with authentication_type=instance_principal and delegation_token_file; run inside OCI Cloud Shell with OCI_CLI_AUTH=instance_obo_user")
	}
	genericProvider, err := auth.GetGenericConfigurationProvider(p)
	if err != nil {
		return nil, fmt.Errorf("create OCI Cloud Shell provider: %w", err)
	}
	return withRegion(genericProvider, cfg.Region), nil
}

type regionProvider struct {
	common.ConfigurationProvider
	region string
}

func (p regionProvider) Region() (string, error) {
	return p.region, nil
}

func withRegion(provider common.ConfigurationProvider, region string) common.ConfigurationProvider {
	region = strings.TrimSpace(region)
	if region == "" {
		return provider
	}
	return regionProvider{ConfigurationProvider: provider, region: region}
}
