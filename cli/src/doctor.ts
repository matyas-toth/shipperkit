import type { Client } from "./client";
import { resolveApplication } from "./apps";
import { heading, muted, paint, printJson, statusText, table } from "./output";

type Check = { layer: string; state: string; summary: string; observedAt: number };
type Diagnostics = {
  checks: Check[];
  observations: Array<{ kind: string; state: string; summary: string; reference?: string }>;
  failedStage?: string;
  unknowns?: string[];
  changesSinceServing?: string[];
};

export async function doctor(client: Client, reference: string, json = false) {
  const app = await resolveApplication(client, reference);
  const result = await client.get<Diagnostics>(`/applications/${app.id}/diagnostics`);
  if (json) {
    printJson(result);
    return;
  }
  process.stdout.write(heading(`Doctor ${app.name}\n\n`));
  if (!result.checks.length) {
    process.stdout.write(muted("No checks reported.") + "\n");
    return;
  }
  process.stdout.write(table(["Layer", "State", "Summary"], result.checks.map(check => [
    check.layer,
    statusText(check.state),
    check.summary,
  ])) + "\n");
  const failed = result.checks.filter(check => check.state === "failed").length;
  const unknownCount = result.checks.filter(check => check.state === "unknown").length;
  if (result.changesSinceServing?.length) {
    process.stdout.write("\n" + heading("Changes since the serving release") + "\n");
    for (const change of result.changesSinceServing) process.stdout.write(muted("• ") + change + "\n");
  }
  if (result.unknowns?.length) {
    process.stdout.write("\n" + heading("Not measured") + "\n");
    for (const item of result.unknowns) process.stdout.write(muted("• ") + item + "\n");
  }
  process.stdout.write("\n");
  if (failed) {
    process.stdout.write(paint().red(`${failed} checks failed`) + "\n");
    process.exitCode = 1;
  } else if (result.failedStage) {
    process.stdout.write(paint().yellow(`A recorded deployment failed at ${result.failedStage}`) + "\n");
  } else if (unknownCount) {
    process.stdout.write(muted(`${unknownCount} checks are unknown`) + "\n");
  } else {
    process.stdout.write(statusText("All measured checks pass") + "\n");
  }
}
