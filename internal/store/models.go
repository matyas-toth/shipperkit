package store

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

type Session struct {
	User      User
	CSRF      string
	ExpiresAt int64
}

type Port struct {
	IP       string `json:"ip"`
	Private  uint16 `json:"private"`
	Public   uint16 `json:"public"`
	Protocol string `json:"protocol"`
}

type Container struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Image         string `json:"image"`
	State         string `json:"state"`
	Status        string `json:"status"`
	Health        string `json:"health"`
	CreatedAt     int64  `json:"createdAt"`
	Ports         []Port `json:"ports"`
	Managed       bool   `json:"managed"`
	ApplicationID string `json:"applicationId,omitempty"`
	Project       string `json:"project,omitempty"`
	Service       string `json:"service,omitempty"`
}

type Application struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	SourceKind string      `json:"sourceKind"`
	SourceRef  string      `json:"sourceRef"`
	Method     string      `json:"method"`
	State      string      `json:"state"`
	CreatedAt  int64       `json:"createdAt"`
	UpdatedAt  int64       `json:"updatedAt"`
	Containers []Container `json:"containers"`
}
