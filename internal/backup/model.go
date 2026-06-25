package backup

import "time"

const CurrentVersion = "1.0"

type File struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Cluster   Cluster   `json:"cluster"`
	Users     []User    `json:"users"`
	ACLs      []ACL     `json:"acls"`
}

type Cluster struct {
	BootstrapServers []string `json:"bootstrap_servers"`
}

type User struct {
	Name        string       `json:"name"`
	Credentials []Credential `json:"credentials"`
}

type Credential struct {
	Mechanism  string `json:"mechanism"`
	Iterations int32  `json:"iterations"`
}

type ACL struct {
	ResourceType        string `json:"resource_type"`
	ResourceName        string `json:"resource_name"`
	ResourcePatternType string `json:"resource_pattern_type"`
	Principal           string `json:"principal"`
	Host                string `json:"host"`
	Operation           string `json:"operation"`
	PermissionType      string `json:"permission_type"`
}
