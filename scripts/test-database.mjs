import { spawnSync } from "node:child_process";
import { goCommand } from "./go.mjs";

const result = spawnSync(goCommand(), ["test", "./internal/deploy", "-run", "TestDockerDatabaseConnectionBetweenApplications", "-v", "-count=1", "-timeout=4m"], {
  env: { ...process.env, SHIPPER_TEST_DATABASE: "1" }, stdio: "inherit", windowsHide: true,
});
process.exitCode = result.status ?? 1;
