import { spawnSync } from "node:child_process";
import { goCommand } from "./go.mjs";
const result = spawnSync(goCommand(), ["test", "./internal/deploy", "-run", "TestDockerComposeApplications", "-v", "-count=1", "-timeout=15m"], { env: { ...process.env, SHIPPER_TEST_COMPOSE: "1" }, stdio: "inherit", windowsHide: true });
process.exitCode = result.status ?? 1;
