import { describe, it, expect } from "vitest";
import { filterContainers, portLabel, statusOf, bytes } from "./data";
import type { Container } from "./types";
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

