export type Role = "admin" | "developer" | "viewer";
export interface User { id: string; email: string; role: Role; hasPassword?: boolean; disabled?: boolean; passwordChangeRequired?: boolean; factorEnabled?: boolean; createdAt?: number; lastLoginAt?: number; identities?: Identity[] }
export interface Identity { provider: "google" | "github" | "discord"; email?: string; createdAt: number; lastLoginAt: number }
export interface Session { setupRequired: boolean; user: User | null; csrfToken?: string; authStep?: "password_change" | "enroll" | "verify"; factor?: { enrolled: boolean; required: boolean }; recoveryCodes?: string[] }
export interface AuthMethods { password: boolean; google: boolean; github: boolean; discord: boolean }
export interface AuthProviderSettings { provider: "google" | "github" | "discord"; enabled: boolean; configured: boolean; clientId: string; hasSecret: boolean; callback: string }
export interface AuthSettings { passwordEnabled: boolean; origin: string; providers: AuthProviderSettings[] }
export interface AuditEvent { sequence: number; userId: string; actorId?: string; action: string; detail?: string; createdAt: number }
export interface Port { ip: string; private: number; public: number; protocol: string }
export interface Container {
  id: string; name: string; image: string; state: string; status: string; health: string;
  createdAt: number; ports: Port[]; managed: boolean; applicationId?: string;
  project?: string; service?: string;
}
export interface DockerHealth {
  connected: boolean; checkedAt: number; observedAt: number; error?: string;
  engine: { name: string; version: string; os: string; architecture: string; cpus: number; memory: number };
}
export interface Variable { name: string; value: string; secret: boolean; scope: "runtime" | "build" | "both"; keep?: boolean; keepFrom?: string; hasValue?: boolean }
export interface PersistentVolume { name: string; path: string }
export interface RuntimePolicy { cpus?:number; pids?:number; restart?:string; startupSeconds?:number; stopSeconds?:number; drainSeconds?:number; compatibility?:boolean }
export interface ServiceConfig { runtime?:RuntimePolicy; name: string; port?: number; healthPath: string; memoryMB: number; variables: Variable[]; unsetVariables?: string[] }
export interface Service { oneShot?: boolean; name: string; image: string; port: number; healthPath: string; volumes: PersistentVolume[]; variables?: Variable[] }
export interface Domain { pathPrefix?:string; stripPrefix?:boolean; compress?:boolean; maintenance?:boolean; responseHeaders?:Record<string,string>; certificateExpiresAt?:number; certificateIssuer?:string; service?: string; id: string; hostname: string; https: boolean; primary: boolean; redirect: boolean; verificationState?: string; message?: string; checkedAt?: number }
export interface DatabaseResource { engine: "postgresql" | "mysql" | "mariadb" | "mongodb" | "clickhouse" | "redis" | "sqlite"; version: string; domain?: string; cache?: boolean }
export interface PublishedPort { service?: string; protocol: "tcp" | "udp"; bindIP: string; hostPort: number; containerPort: number }
export interface NetworkAccess { ports: PublishedPort[] }
export interface NetworkingInfo { networking: NetworkAccess; host: string }
export interface ReviewSummaryItem { label: string; value: string }
export interface TerminalAudit { sequence: number; sessionId: string; userId: string; applicationId: string; containerId: string; event: string; reason: string; createdAt: number }
export interface StaticConfig { networking?: NetworkAccess; database?: DatabaseResource; commandsKeep?:boolean; localSource?:string; volumeSources?:Record<string,string>; runtime?:RuntimePolicy; sourceId?:string; repositoryId?:number; autoDeploy?:{enabled:boolean;paths?:string[];supersede:boolean}; image?:string; registryId?:string; platform?:string; inlineDockerfile?:string; inlineKeep?:boolean; inlineHasValue?:boolean; hooks?: Hook[]; startupPolicy?: "http" | "process"; repository: string; branch: string; publishDirectory: string; routing: string; htaccess: boolean; domain: string; https: boolean; method: "auto" | "static" | "dockerfile" | "railpack" | "compose" | "image" | "inline"; rootDirectory: string; dockerfile: string; installCommand: string; buildCommand: string; startCommand: string; port: number; healthPath: string; variables: Variable[]; volumes: PersistentVolume[]; memoryMB: number; domains?: Domain[]; composeSource?: "git" | "paste"; composeFile?: string; composeYAML?: string; composeKeep?: boolean; composeHasValue?: boolean; catalogService?: string; catalogVersion?: string; services?: ServiceConfig[] }
export interface CatalogField { key:string; label:string; kind:"text"|"password"|"number"|"boolean"|"select"|"url"|"email"|"textarea"|"domain"; group:"access"|"network"|"configuration"|"advanced"; default?:string; placeholder?:string; help?:string; options?:string[]; service?:string; required?:boolean; secret?:boolean; generate?:"password"|"base64"|"hex"; advanced?:boolean }
export interface CatalogServiceInfo { name:string; image:string; role:string; port?:number }
export interface CatalogDomain { key?:string; service:string; port:number; primary?:boolean }
export interface CatalogServiceSummary { id:string; name:string; description:string; category:string; tags?:string[]; logomark:string; logo?:string; website?:string; documentation?:string; source:"coolify"|"shipper"; featured?:boolean; mainService:string; port:number; support:"ready"|"advanced"; supportNote?:string; services:number; fields:number; domain:boolean }
export interface CatalogService extends Omit<CatalogServiceSummary,"services"|"fields"|"domain"> { updated?:string; healthPath?:string; services:CatalogServiceInfo[]; fields:CatalogField[]; domains?:CatalogDomain[]; compose:string }
export interface CatalogIndex { version:string; source:string; generated:string; updatedAt:number; checkedAt:number; fromCache?:boolean; error?:string; categories:{name:string;count:number}[]; items:CatalogServiceSummary[] }
export interface Deployment { rollbackId?:string; digest?:string;cancelRequested?:boolean; actor?: string; taskId?: string; services?: Service[]; applicationName: string; id: string; applicationId: string; operation: string; state: string; config: StaticConfig; commit: string; imageId: string; containerId: string; previousId: string; error: string; createdAt: number; startedAt: number; finishedAt: number; resolvedMethod: string; runtimePort: number; noCache: boolean }
export interface Application { id: string; name: string; sourceKind: string; sourceRef: string; method: string; state: string; createdAt: number; updatedAt: number; containers: Container[]; config: StaticConfig; activeDeploymentId: string; latestDeployment?: Deployment; domains: Domain[]; desiredState: string; activeMethod?: string }
export interface ApplicationDetail { application: Application; deployments: Deployment[]; activeDeployment?: Deployment | null; database?: { engine: string; host: string; port: number } | null }
export interface LogLine { number: number; sequence: number; step: string; line: string; createdAt: number }
export interface ProxyConfig { dashboardURL?:string; dashboardUpstream?:string; bindIP: string; httpPort: number; httpsPort: number; acmeEmail: string; acmeCA: string; autoHTTPS?: boolean; publicAddress?: string; appDomainSuffix?: string }
export interface Inventory { items: Application[]; docker: DockerHealth }
export interface SystemInfo {
  version: string; instanceId: string; docker: DockerHealth;
  database: { engine: string; journal: string; counts: Record<string, number> };
  runtime: { go: string; heapBytes: number; goroutines: number };
}
export interface OverviewValue { percent: number | null; used?: number; total?: number }
export interface OverviewSeries { from: number; to: number; step: number; cpu: Array<number | null>; memory: Array<number | null>; disk: Array<number | null> }
export interface SystemOverview {
  sampledAt: number; stale: boolean; vantage: string;
  cpu: OverviewValue; memory: OverviewValue; disk: OverviewValue;
  load1: number; containers: { running: number; total: number };
  series: OverviewSeries; thresholds: { cpu: string; memory: string; disk: string };
  measuring: boolean; unavailableCpu: boolean;
}
export interface DiagnosisCause { code: string; summary: string; detail: string; confidence: "high" | "medium" | "low" | string; evidence: string[] }
export interface DiagnosisCheck { layer: string; state: string; summary: string; observedAt: number }
export interface DiagnosisService { name: string; image?: string; state: string; health: string; exitCode: number; restarts: number; oomKilled: boolean; cpuPercent?: number; memoryBytes?: number; memoryLimit?: number; runtimePort?: number; healthPath?: string }
export interface DiagnosisLog { service: string; lines: string[] }
export interface DiagnosisAction { id: string; label: string; kind: "navigate" | "retry" | "copy" | "docs" | string; target: string }
export interface DeploymentDiagnosis {
  schemaVersion: number; generatedAt: number; vantage: string;
  application: { id: string; name: string; method: string };
  deployment: { id: string; operation: string; state: string; stage: string; durationSeconds: number; createdAt: number; error?: string };
  cause: DiagnosisCause; checks: DiagnosisCheck[]; services: DiagnosisService[];
  logs: DiagnosisLog[]; changesSinceServing: string[]; unknowns: string[]; nextActions: DiagnosisAction[];
}

export type UpdateChannel = "main" | "nightly";
export interface UpdateAvailable { version: string; image: string; releaseUrl: string; channel: UpdateChannel; checkedAt: number }
export interface UpdateStatus { state: string; message: string; fromVersion: string; toVersion: string; startedAt: number; finishedAt: number; requestId: string; log: string }
export interface UpdateState {
  enabled: boolean; currentVersion: string; latestVersion?: string; channel: UpdateChannel; auto: boolean;
  available: UpdateAvailable | null; checkedAt: number; checking: boolean; error?: string; status: UpdateStatus;
}


export interface Command { shell: string; argv?: string[]; service: string; directory: string; user: string; timeout: number; mode: "job" | "exec" }
export interface Hook { stage: "before_deploy" | "before_start" | "after_deploy" | "before_stop"; command: Command }
export interface SavedTask { id: string; applicationId: string; name: string; command: Command; enabled: boolean; schedule: string; timezone: string; overlap: "skip" | "queue"; missed: "skip" | "once"; nextAt: number }
export interface TaskRun { id: string; deploymentId: string; stage: string; state: string; command: Command; containerId: string; execId: string; exitCode: number; error: string; startedAt: number; finishedAt: number }
export interface ChangePlan { effects?:string[]; downtime?:string; operation: "none" | "recreate" | "rebuild" | "metadata" | "route" | "rollback"; changes: string[]; revision: string }

export interface Connection {username?:string;allowPrivate?:boolean;id:string;name:string;kind:"github_app"|"github_token"|"registry";host:string;updatedAt:number}
