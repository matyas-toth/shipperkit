export interface User { id: string; email: string }
export interface Session { setupRequired: boolean; user: User | null; csrfToken?: string; authStep?: "enroll" | "verify"; factor?: { enrolled: boolean; required: boolean }; recoveryCodes?: string[] }
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
export interface RuntimePolicy { cpus?:number; pids?:number; restart?:string; startupSeconds?:number; stopSeconds?:number; drainSeconds?:number }
export interface ServiceConfig { runtime?:RuntimePolicy; name: string; port?: number; healthPath: string; memoryMB: number; variables: Variable[]; unsetVariables?: string[] }
export interface Service { oneShot?: boolean; name: string; image: string; port: number; healthPath: string; volumes: PersistentVolume[]; variables?: Variable[] }
export interface Domain { pathPrefix?:string; stripPrefix?:boolean; compress?:boolean; maintenance?:boolean; responseHeaders?:Record<string,string>; certificateExpiresAt?:number; certificateIssuer?:string; service?: string; id: string; hostname: string; https: boolean; primary: boolean; redirect: boolean; verificationState?: string; message?: string; checkedAt?: number }
export interface StaticConfig { commandsKeep?:boolean; localSource?:string; volumeSources?:Record<string,string>; runtime?:RuntimePolicy; sourceId?:string; repositoryId?:number; autoDeploy?:{enabled:boolean;paths?:string[];supersede:boolean}; image?:string; registryId?:string; platform?:string; inlineDockerfile?:string; inlineKeep?:boolean; inlineHasValue?:boolean; hooks?: Hook[]; startupPolicy?: "http" | "process"; repository: string; branch: string; publishDirectory: string; routing: string; htaccess: boolean; domain: string; https: boolean; method: "auto" | "static" | "dockerfile" | "railpack" | "compose" | "image" | "inline"; rootDirectory: string; dockerfile: string; installCommand: string; buildCommand: string; startCommand: string; port: number; healthPath: string; variables: Variable[]; volumes: PersistentVolume[]; memoryMB: number; domains?: Domain[]; composeSource?: "git" | "paste"; composeFile?: string; composeYAML?: string; composeKeep?: boolean; composeHasValue?: boolean; services?: ServiceConfig[] }
export interface Deployment { rollbackId?:string; digest?:string;cancelRequested?:boolean; actor?: string; taskId?: string; services?: Service[]; applicationName: string; id: string; applicationId: string; operation: string; state: string; config: StaticConfig; commit: string; imageId: string; containerId: string; previousId: string; error: string; createdAt: number; startedAt: number; finishedAt: number; resolvedMethod: string; runtimePort: number; noCache: boolean }
export interface Application { id: string; name: string; sourceKind: string; sourceRef: string; method: string; state: string; createdAt: number; updatedAt: number; containers: Container[]; config: StaticConfig; activeDeploymentId: string; latestDeployment?: Deployment; domains: Domain[]; desiredState: string; activeMethod?: string }
export interface ApplicationDetail { application: Application; deployments: Deployment[]; activeDeployment?: Deployment | null; database?: { engine: string; host: string; port: number } | null }
export interface LogLine { number: number; sequence: number; step: string; line: string; createdAt: number }
export interface ProxyConfig { dashboardURL?:string; dashboardUpstream?:string; bindIP: string; httpPort: number; httpsPort: number; acmeEmail: string; acmeCA: string; autoHTTPS?: boolean }
export interface Inventory { items: Application[]; docker: DockerHealth }
export interface SystemInfo {
  version: string; instanceId: string; docker: DockerHealth;
  database: { engine: string; journal: string; counts: Record<string, number> };
  runtime: { go: string; heapBytes: number; goroutines: number };
}

export type UpdateChannel = "main" | "nightly";
export interface UpdateAvailable { version: string; image: string; releaseUrl: string; channel: UpdateChannel; checkedAt: number }
export interface UpdateStatus { state: string; message: string; fromVersion: string; toVersion: string; startedAt: number; finishedAt: number; requestId: string; log: string }
export interface UpdateState {
  enabled: boolean; currentVersion: string; channel: UpdateChannel; auto: boolean;
  available: UpdateAvailable | null; checkedAt: number; checking: boolean; error?: string; status: UpdateStatus;
}


export interface Command { shell: string; argv?: string[]; service: string; directory: string; user: string; timeout: number; mode: "job" | "exec" }
export interface Hook { stage: "before_deploy" | "before_start" | "after_deploy" | "before_stop"; command: Command }
export interface SavedTask { id: string; applicationId: string; name: string; command: Command; enabled: boolean; schedule: string; timezone: string; overlap: "skip" | "queue"; missed: "skip" | "once"; nextAt: number }
export interface TaskRun { id: string; deploymentId: string; stage: string; state: string; command: Command; containerId: string; execId: string; exitCode: number; error: string; startedAt: number; finishedAt: number }
export interface ChangePlan { effects?:string[]; downtime?:string; operation: "none" | "recreate" | "rebuild" | "metadata" | "rollback"; changes: string[]; revision: string }

export interface Connection {username?:string;allowPrivate?:boolean;id:string;name:string;kind:"github_app"|"github_token"|"registry";host:string;updatedAt:number}
