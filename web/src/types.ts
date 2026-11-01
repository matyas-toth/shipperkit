export interface User { id: string; email: string }
export interface Session { setupRequired: boolean; user: User | null; csrfToken?: string }
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
export interface StaticConfig { repository: string; branch: string; publishDirectory: string; routing: string; htaccess: boolean; domain: string; https: boolean }
export interface Deployment { id: string; applicationId: string; operation: string; state: string; config: StaticConfig; commit: string; imageId: string; containerId: string; previousId: string; error: string; createdAt: number; startedAt: number; finishedAt: number }
export interface Application { id: string; name: string; sourceKind: string; sourceRef: string; method: string; state: string; createdAt: number; updatedAt: number; containers: Container[]; config: StaticConfig; activeDeploymentId: string; latestDeployment?: Deployment }
export interface ApplicationDetail { application: Application; deployments: Deployment[] }
export interface LogLine { sequence: number; step: string; line: string; createdAt: number }
export interface ProxyConfig { bindIP: string; httpPort: number; httpsPort: number; acmeEmail: string; acmeCA: string }
export interface Inventory { items: Application[]; docker: DockerHealth }
export interface SystemInfo {
  version: string; instanceId: string; docker: DockerHealth;
  database: { engine: string; journal: string; counts: Record<string, number> };
  runtime: { go: string; heapBytes: number; goroutines: number };
}

