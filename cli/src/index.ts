#!/usr/bin/env node
import { Command } from "commander";
import { login, logout, status } from "./auth";
import { Client } from "./client";
import { resolveAuth } from "./config";
import { create, deploy, lifecycle, openApplication, remove, resolveApplication, showApp, showApps } from "./apps";
import { deployLocal } from "./local";
import { existsSync, statSync } from "node:fs";
import { doctor } from "./doctor";
import { logs } from "./logs";
import { api } from "./api";
import { fail, setColor } from "./output";

const version = "0.1.0";

type GlobalOptions = {
  json?: boolean;
  color?: boolean;
  url?: string;
  token?: string;
};

const program = new Command();
program
  .name("shipper")
  .description("Control a Shipper instance from your terminal.")
  .version(version)
  .option("--json", "Print machine-readable JSON")
  .option("--no-color", "Disable color output")
  .option("--url <url>", "Override the stored instance URL")
  .option("--token <token>", "Override the stored token")
  .showHelpAfterError();

function globals(command: Command): GlobalOptions {
  return command.optsWithGlobals() as GlobalOptions;
}

function client(command: Command) {
  const options = globals(command);
  return new Client(resolveAuth({ url: options.url, token: options.token }));
}

function json(command: Command) {
  return Boolean(globals(command).json);
}

function configure(command: Command) {
  const options = globals(command);
  const terminal = Boolean(process.stdout.isTTY) || Boolean(process.env.FORCE_COLOR && process.env.FORCE_COLOR !== "0");
  setColor(options.color !== false && !process.env.NO_COLOR && terminal);
}

const auth = program.command("auth").description("Sign in to a Shipper instance");
auth
  .command("login")
  .description("Authorize this computer in the browser")
  .option("--url <url>", "Instance URL")
  .option("--name <name>", "Name shown in the access token list")
  .action(async (options, command) => {
    configure(command);
    await login({ url: options.url || globals(command).url, name: options.name }, json(command));
  });
auth
  .command("status")
  .description("Show the current instance and token")
  .action(async (_options, command) => {
    configure(command);
    await status({ url: globals(command).url, token: globals(command).token }, json(command));
  });
auth
  .command("logout")
  .description("Remove the local token")
  .action((_options, command) => {
    configure(command);
    logout(json(command));
  });

const apps = program.command("apps").description("List applications");
apps.action(async (_options, command) => {
  configure(command);
  await showApps(client(command), json(command));
});
apps
  .command("create <name>")
  .description("Create an application from a Git repository")
  .requiredOption("--repo <url>", "Git repository URL")
  .option("--branch <branch>", "Git branch", "main")
  .option("--domain <domain>", "Public domain")
  .option("--method <method>", "Build method", "auto")
  .action(async (name, options, command) => {
    configure(command);
    await create(client(command), name, { repo: options.repo, branch: options.branch, domain: options.domain, method: options.method }, json(command));
  });

program
  .command("app <name>")
  .description("Show one application")
  .action(async (name, _options, command) => {
    configure(command);
    await showApp(client(command), name, json(command));
  });

program
  .command("deploy <app|path>")
  .description("Deploy a local folder, or deploy an application again")
  .option("--no-cache", "Rebuild without the Docker cache")
  .option("--name <name>", "Application name for a local folder")
  .option("--domain <domain>", "Public domain for a local folder")
  .option("--method <method>", "Build method for a local folder")
  .action(async (name, options, command) => {
    configure(command);
    if (existsSync(name) && statSync(name).isDirectory()) {
      await deployLocal(client(command), name, { name: options.name, domain: options.domain, method: options.method }, json(command));
    } else {
      await deploy(client(command), name, options.cache === false, json(command));
    }
  });

program
  .command("logs <app>")
  .description("Stream runtime logs")
  .option("--tail <lines>", "Historical lines requested from the container", value => parseInt(value, 10), 200)
  .option("--lines <lines>", "Lines to print before stopping", value => parseInt(value, 10), 100)
  .option("--follow", "Keep streaming")
  .action(async (name, options, command) => {
    configure(command);
    await logs(client(command), name, { tail: options.tail, lines: options.lines, follow: options.follow });
  });

program
  .command("doctor <app>")
  .description("Show diagnostics for an application")
  .action(async (name, _options, command) => {
    configure(command);
    await doctor(client(command), name, json(command));
  });

for (const action of ["start", "stop", "restart"] as const) {
  program
    .command(`${action} <app>`)
    .description(`${action[0].toUpperCase() + action.slice(1)} an application`)
    .action(async (name, _options, command) => {
      configure(command);
      await lifecycle(client(command), name, action, json(command));
    });
}

program
  .command("delete <app>")
  .description("Remove an application")
  .option("--yes", "Skip the confirmation flag check")
  .action(async (name, options, command) => {
    configure(command);
    await remove(client(command), name, Boolean(options.yes), json(command));
  });

program
  .command("open <app>")
  .description("Open the application domain in the browser")
  .action(async (name, _options, command) => {
    configure(command);
    const app = await resolveApplication(client(command), name);
    const url = openApplication(app);
    process.stdout.write(url + "\n");
  });

program
  .command("api <method> <path>")
  .description("Call the authenticated v1 API with JSON output")
  .option("--body <json>", "JSON body, or - to read stdin")
  .action(async (method, path, options, command) => {
    configure(command);
    await api(client(command), method, path, options.body);
  });

program.addHelpText("after", `
Examples:
  shipper auth login
  shipper apps
  shipper app Website
  shipper deploy .
  shipper deploy . --name atlas
  shipper deploy Website
  shipper deploy Website --no-cache
  shipper logs Website --lines 50
  shipper doctor Website
  shipper restart Website
  shipper apps create "Docs" --repo https://github.com/acme/docs --domain docs.example.com
  shipper api GET /applications
  shipper api POST /applications/<id>/lifecycle --body '{"action":"restart"}'

Environment:
  SHIPPER_URL, SHIPPER_TOKEN   Use credentials without a local login.
  SHIPPER_CONFIG               Custom config file path.
`);

program.parseAsync(process.argv).catch((error: Error & { code?: string; status?: number }) => {
  if (program.opts<GlobalOptions>().json) {
    process.stderr.write(JSON.stringify({ error: { message: error.message, code: error.code || "", status: error.status || 0 } }) + "\n");
  } else {
    fail(error.message);
  }
  process.exitCode = 1;
});
