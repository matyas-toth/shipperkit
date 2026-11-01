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
	DeploymentID  string `json:"deploymentId,omitempty"`
}

type Application struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	SourceKind         string       `json:"sourceKind"`
	SourceRef          string       `json:"sourceRef"`
	Method             string       `json:"method"`
	State              string       `json:"state"`
	CreatedAt          int64        `json:"createdAt"`
	UpdatedAt          int64        `json:"updatedAt"`
	Containers         []Container  `json:"containers"`
	Config             StaticConfig `json:"config"`
	ActiveDeploymentID string       `json:"activeDeploymentId"`
	LatestDeployment   *Deployment  `json:"latestDeployment,omitempty"`
}

type StaticConfig struct {
	Repository       string `json:"repository"`
	Branch           string `json:"branch"`
	PublishDirectory string `json:"publishDirectory"`
	Routing          string `json:"routing"`
	HTAccess         bool   `json:"htaccess"`
	Domain           string `json:"domain"`
	HTTPS            bool   `json:"https"`
}

type Deployment struct {
	ID            string       `json:"id"`
	ApplicationID string       `json:"applicationId"`
	Operation     string       `json:"operation"`
	State         string       `json:"state"`
	Config        StaticConfig `json:"config"`
	Commit        string       `json:"commit"`
	ImageID       string       `json:"imageId"`
	ContainerID   string       `json:"containerId"`
	PreviousID    string       `json:"previousId"`
	Error         string       `json:"error"`
	CreatedAt     int64        `json:"createdAt"`
	StartedAt     int64        `json:"startedAt"`
	FinishedAt    int64        `json:"finishedAt"`
}
type LogLine struct {
	Sequence  int64  `json:"sequence"`
	Step      string `json:"step"`
	Line      string `json:"line"`
	CreatedAt int64  `json:"createdAt"`
}
