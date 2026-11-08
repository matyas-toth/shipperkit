import { describe, it, expect } from "vitest";
import { filterContainers, portLabel, statusOf, bytes, deploymentSeconds, formatDuration, applicationState } from "./data";
import type { Container, Deployment, Application } from "./types";
const running: Container = { id: "abc", name: "API", image: "example/api:v1", state: "running", health: "", status: "Up", createdAt: 0, ports: [], managed: true };
describe("inventory presentation", () => {
  it("keeps health distinct from process state", () => {
    expect(statusOf({ ...running, health: "unhealthy" })).toBe("unhealthy");
    expect(statusOf({ ...running, health: "starting" })).toBe("starting");
  });
  it("combines ownership, status, and case-insensitive local search", () => {
    const external = { ...running, id: "def", name: "database", managed: false };
    expect(filterContainers([running, external], " EXAMPLE/API ", "managed", "running")).toEqual([running]);
    expect(filterContainers([external], "", "managed", "all")).toEqual([]);
  });
  it("formats published and internal ports without suggesting every port is public", () => {
    expect(portLabel([{ ip: "127.0.0.1", public: 8080, private: 80, protocol: "tcp" }, { ip: "", public: 0, private: 5432, protocol: "tcp" }])).toBe("8080:80/tcp, 5432/tcp");
    expect(bytes(1024 ** 3)).toBe("1.0 GiB");
  });
});


describe("deployment history", () => {
  it("counts elapsed queue/build time live and freezes terminal jobs", () => {
    const deployment = { state: "building", createdAt:100, startedAt:102, finishedAt:0 } as Deployment;
    expect(deploymentSeconds(deployment,115)).toBe(15);
    expect(deploymentSeconds(deployment,116)).toBe(16);
    expect(deploymentSeconds({...deployment,state:"failed",finishedAt:120},500)).toBe(20);
    expect(deploymentSeconds(deployment,90)).toBe(0);
    expect(formatDuration(65)).toBe("1m 05s");expect(formatDuration(3665)).toBe("1h 01m 05s");
  });
  it("shows serving static releases without inventing availability for absent containers", () => {
    const app = { method:"auto",activeMethod:"static",sourceKind:"git",activeDeploymentId:"release",state:"running" } as Application;
    expect(applicationState(app)).toBe("serving");expect(applicationState({...app,state:"missing"})).toBe("unavailable");
    expect(applicationState({...app,latestDeployment:{state:"building"} as Deployment})).toBe("serving");
    expect(applicationState({...app,latestDeployment:{state:"failed"} as Deployment})).toBe("serving");
    expect(applicationState({...app,activeDeploymentId:"",latestDeployment:{state:"building"} as Deployment})).toBe("building");
    expect(applicationState({...app,desiredState:"stopped"})).toBe("stopped");
  });
});
