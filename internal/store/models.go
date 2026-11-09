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
	Factor    FactorState
	Verified  bool
}

func (a Application) Managed() bool {
	if a.Method == "" {
		return false
	}
	switch a.SourceKind {
	case "git", "compose", "image", "inline", "local":
		return true
	}
	return false
}

func (s Session) AuthStep() string {
	if s.Factor.Enrolled && !s.Verified {
		return "verify"
	}
	if s.Factor.Required && !s.Factor.Enrolled {
		return "enroll"
	}
	return ""
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
	Domains            []Domain     `json:"domains"`
	DesiredState       string       `json:"desiredState"`
	ActiveMethod       string       `json:"activeMethod"`
}

type Command struct {
	Shell     string   `json:"shell"`
	Argv      []string `json:"argv,omitempty"`
	Service   string   `json:"service"`
	Directory string   `json:"directory"`
	User      string   `json:"user"`
	Timeout   int      `json:"timeout"`
	Mode      string   `json:"mode"`
}
type Hook struct {
	Stage   string  `json:"stage"`
	Command Command `json:"command"`
}

type AutoDeploy struct {
	Enabled   bool     `json:"enabled"`
	Paths     []string `json:"paths,omitempty"`
	Supersede bool     `json:"supersede"`
}
type RuntimePolicy struct {
	CPUs           float64 `json:"cpus,omitempty"`
	PIDs           int64   `json:"pids,omitempty"`
	Restart        string  `json:"restart,omitempty"`
	StartupSeconds int     `json:"startupSeconds,omitempty"`
	StopSeconds    int     `json:"stopSeconds,omitempty"`
	DrainSeconds   int     `json:"drainSeconds,omitempty"`
}
type StaticConfig struct {
	Database      *DatabaseResource `json:"database,omitempty"`
	Networking    NetworkAccess     `json:"networking,omitempty"`
	CommandsKeep  bool              `json:"commandsKeep,omitempty"`
	LocalSource   string            `json:"localSource,omitempty"`
	VolumeSources map[string]string `json:"volumeSources,omitempty"`
	Runtime       RuntimePolicy     `json:"runtime"`

	SourceID         string          `json:"sourceId,omitempty"`
	RepositoryID     int64           `json:"repositoryId,omitempty"`
	AutoDeploy       AutoDeploy      `json:"autoDeploy"`
	Image            string          `json:"image,omitempty"`
	RegistryID       string          `json:"registryId,omitempty"`
	Platform         string          `json:"platform,omitempty"`
	InlineDockerfile string          `json:"inlineDockerfile,omitempty"`
	InlineKeep       bool            `json:"inlineKeep,omitempty"`
	InlineHasValue   bool            `json:"inlineHasValue,omitempty"`
	Hooks            []Hook          `json:"hooks,omitempty"`
	StartupPolicy    string          `json:"startupPolicy,omitempty"`
	Repository       string          `json:"repository"`
	Branch           string          `json:"branch"`
	PublishDirectory string          `json:"publishDirectory"`
	Routing          string          `json:"routing"`
	HTAccess         bool            `json:"htaccess"`
	Domain           string          `json:"domain"`
	HTTPS            bool            `json:"https"`
	Method           string          `json:"method"`
	RootDirectory    string          `json:"rootDirectory"`
	Dockerfile       string          `json:"dockerfile"`
	InstallCommand   string          `json:"installCommand"`
	BuildCommand     string          `json:"buildCommand"`
	StartCommand     string          `json:"startCommand"`
	Port             int             `json:"port"`
	HealthPath       string          `json:"healthPath"`
	Variables        []Variable      `json:"variables"`
	Volumes          []Volume        `json:"volumes"`
	Domains          []Domain        `json:"domains,omitempty"`
	MemoryMB         int64           `json:"memoryMB"`
	ComposeSource    string          `json:"composeSource"`
	ComposeFile      string          `json:"composeFile"`
	ComposeYAML      string          `json:"composeYAML"`
	ComposeKeep      bool            `json:"composeKeep,omitempty"`
	ComposeHasValue  bool            `json:"composeHasValue,omitempty"`
	Services         []ServiceConfig `json:"services"`
}
type DatabaseResource struct {
	Engine      string `json:"engine"`
	Version     string `json:"version"`
	TLSHostname string `json:"tlsHostname,omitempty"`
}
type ServiceConfig struct {
	Runtime        RuntimePolicy `json:"runtime"`
	Name           string        `json:"name"`
	Port           *int          `json:"port,omitempty"`
	HealthPath     string        `json:"healthPath"`
	MemoryMB       int64         `json:"memoryMB"`
	Variables      []Variable    `json:"variables"`
	UnsetVariables []string      `json:"unsetVariables,omitempty"`
}
type Service struct {
	OneShot    bool       `json:"oneShot"`
	Name       string     `json:"name"`
	Image      string     `json:"image"`
	Port       int        `json:"port"`
	HealthPath string     `json:"healthPath"`
	Volumes    []Volume   `json:"volumes"`
	Variables  []Variable `json:"variables"`
}
type Config = StaticConfig
type Variable struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Secret   bool   `json:"secret"`
	Scope    string `json:"scope"`
	Keep     bool   `json:"keep,omitempty"`
	KeepFrom string `json:"keepFrom,omitempty"`
	HasValue bool   `json:"hasValue,omitempty"`
}
type Volume struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type Domain struct {
	PathPrefix           string            `json:"pathPrefix,omitempty"`
	StripPrefix          bool              `json:"stripPrefix,omitempty"`
	Compress             bool              `json:"compress,omitempty"`
	Maintenance          bool              `json:"maintenance,omitempty"`
	ResponseHeaders      map[string]string `json:"responseHeaders,omitempty"`
	CertificateExpiresAt int64             `json:"certificateExpiresAt,omitempty"`
	CertificateIssuer    string            `json:"certificateIssuer,omitempty"`

	Service           string `json:"service,omitempty"`
	ID                string `json:"id"`
	Hostname          string `json:"hostname"`
	HTTPS             bool   `json:"https"`
	Primary           bool   `json:"primary"`
	Redirect          bool   `json:"redirect"`
	VerificationState string `json:"verificationState,omitempty"`
	Message           string `json:"message,omitempty"`
	CheckedAt         int64  `json:"checkedAt,omitempty"`
}

type Deployment struct {
	RollbackID      string       `json:"rollbackId,omitempty"`
	Digest          string       `json:"digest,omitempty"`
	CancelRequested bool         `json:"cancelRequested,omitempty"`
	ID              string       `json:"id"`
	ApplicationName string       `json:"applicationName"`
	ApplicationID   string       `json:"applicationId"`
	Operation       string       `json:"operation"`
	State           string       `json:"state"`
	Config          StaticConfig `json:"config"`
	Commit          string       `json:"commit"`
	ImageID         string       `json:"imageId"`
	ContainerID     string       `json:"containerId"`
	PreviousID      string       `json:"previousId"`
	Error           string       `json:"error"`
	CreatedAt       int64        `json:"createdAt"`
	StartedAt       int64        `json:"startedAt"`
	FinishedAt      int64        `json:"finishedAt"`
	PreviousDomains []Domain     `json:"previousDomains,omitempty"`
	Service         string       `json:"service,omitempty"`
	Actor           string       `json:"actor,omitempty"`
	TaskID          string       `json:"taskId,omitempty"`
	TaskCommand     *Command     `json:"taskCommand,omitempty"`
	NoCache         bool         `json:"noCache"`
	ResolvedMethod  string       `json:"resolvedMethod"`
	RuntimePort     int          `json:"runtimePort"`
	ComposePlan     string       `json:"-"`
	Services        []Service    `json:"services"`
}
type LogLine struct {
	Sequence  int64  `json:"sequence"`
	Number    int64  `json:"number"`
	Step      string `json:"step"`
	Line      string `json:"line"`
	CreatedAt int64  `json:"createdAt"`
}
