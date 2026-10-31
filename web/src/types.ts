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
export interface Inventory { items: Container[]; docker: DockerHealth }
export interface SystemInfo {
  version: string; docker: DockerHealth;
  database: { engine: string; journal: string; counts: Record<string, number> };
  runtime: { go: string; heapBytes: number; goroutines: number };
}

