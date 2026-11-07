import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {mkdtemp,writeFile,rm} from "node:fs/promises";
import {resolve,sep} from "node:path";

let savedCookies:Awaited<ReturnType<import("@playwright/test").BrowserContext["cookies"]>>|undefined;
const folders:string[]=[];
async function login(page: import("@playwright/test").Page) {
  if(savedCookies){await page.context().addCookies(savedCookies);await page.goto("/");return;}
  await page.goto("/");
  await page.getByLabel("Email", {exact:true}).fill("owner@example.test");
  await page.getByLabel("Password", {exact:true}).fill("shipper-browser-test-password");
  await page.getByRole("button",{name:"Sign in",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Applications",exact:true})).toBeVisible();savedCookies=await page.context().cookies();
}
test.afterEach(async({page})=>{await page.unrouteAll({behavior:"wait"});for(const directory of folders.splice(0)){if(resolve(directory).startsWith(resolve(tmpdir())+sep))await rm(directory,{recursive:true,force:true});}});

async function checkDatabaseConnection(page: import("@playwright/test").Page) {
  const original = await (await page.request.get("/api/applications/application-web")).json();
  const release = { ...original.activeDeployment, id: "database-release", resolvedMethod: "image", runtimePort: 5432, config: { ...original.application.config, method: "image", image: "postgres:17", startupPolicy: "process" } };
  const database = { ...original.application, name: "PostgreSQL", state: "running", activeMethod: "image", method: "image", sourceKind: "image", sourceRef: "postgres:17", config: release.config, activeDeploymentId: release.id, latestDeployment: release, domains: [], containers: [{ ...original.application.containers[0], name: "shp-postgresql-review", state: "running", health: "healthy", image: "postgres:17", ports: [] }] };
  const uri = "postgresql://app:review-password@site-database-release:5432/shop";
  let copies = "", requests = 0, available = true;
  await page.addInitScript(() => Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: async (text: string) => { (window as any).copiedDatabase = text; } } }));
  await page.route("**/api/applications/application-web/diagnostics", route => route.fulfill({ json: { checks: [{ layer: "runtime", state: "healthy", summary: "Container is running", observedAt: Math.floor(Date.now() / 1000) }], servingRelease: release, latestOperation: release, durationSeconds: 15, observations: [], changesSinceServing: [], unknowns: [] } }));
  await page.route("**/api/applications/application-web", route => route.fulfill({ json: { application: database, deployments: [release], activeDeployment: release, database: available ? { engine: "postgresql", host: "site-database-release", port: 5432 } : null } }));
  await page.route("**/api/applications/application-web/database/connection", route => { requests++; return route.fulfill({ json: { uri, deploymentId: release.id } }); });
  await page.goto("/applications/application-web");
  await expect(page.getByLabel("Connection string", { exact: true })).toHaveAttribute("type", "password");
  expect(requests).toBe(0);
  await page.getByRole("button", { name: "Copy connection string", exact: true }).click();
  await expect(page.getByRole("button", { name: "Copy connection string", exact: true })).toHaveText("Copied");
  copies = await page.evaluate(() => (window as any).copiedDatabase);
  expect(copies).toBe(uri);
  await expect(page.getByLabel("Connection string", { exact: true })).not.toHaveValue(uri);
  await page.getByRole("button", { name: "Reveal connection string", exact: true }).click();
  await expect(page.getByLabel("Connection string", { exact: true })).toHaveValue(uri);
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-9-connection.png") });
  await page.getByRole("button", { name: "Hide connection string", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-9-mobile.png") });
  // HTTP installations without Clipboard API can use the copy-command fallback.
  await page.evaluate(() => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    document.execCommand = (command: string) => { (window as any).fallbackDatabase = (document.activeElement as HTMLTextAreaElement).value; return command === "copy"; };
  });
  await page.getByRole("button", { name: "Copy connection string", exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).fallbackDatabase)).toBe(uri);
  available = false;
  await page.getByRole("link", { name: "Applications", exact: true }).first().click();
  await page.goto("/applications/application-web");
  await expect(page.getByRole("region", { name: "Database connection", exact: true })).toHaveCount(0);
  await page.unroute("**/api/applications/application-web");
  await page.unroute("**/api/applications/application-web/database/connection");
  await page.unroute("**/api/applications/application-web/diagnostics");
  await page.setViewportSize({ width: 1440, height: 1100 });
}

test("System tokens, notification choices and responsive diagnostics", async({page})=>{
  await page.setViewportSize({width:1440,height:1100});
  const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
  await login(page);
  await checkDatabaseConnection(page);
  await page.getByRole("link",{name:"System",exact:true}).click();
  await expect(page).toHaveTitle("System | Shipper");await expect(page.getByRole("heading",{name:"Health",exact:true})).toBeVisible();
  await page.getByRole("link",{name:"Access tokens",exact:true}).click();await expect(page.getByRole("heading",{name:"Health",exact:true})).toHaveCount(0);
  await page.getByText("Create token",{exact:true}).first().click();
  await page.getByLabel("Name",{exact:true}).fill("Read-only test automation");
  await page.getByLabel("Application",{exact:true}).selectOption("application-web");
  await page.getByRole("button",{name:"Create token",exact:true}).click();
  await expect(page.getByLabel("New access token")).toHaveValue(/^[a-f0-9]{64}$/);
  const raw=await page.getByLabel("New access token").inputValue();
  const allowed=await page.request.get("/api/v1/applications/application-web/diagnostics",{headers:{Authorization:"Bearer "+raw}});expect(allowed.status()).toBe(200);
  const denied=await page.request.post("/api/v1/applications/application-web/deployments",{headers:{Authorization:"Bearer "+raw},data:{}});expect(denied.status()).toBe(403);
  await page.getByRole("button",{name:"Dismiss",exact:true}).click();await expect(page.getByLabel("New access token")).toHaveCount(0);
  await page.getByRole("row").filter({hasText:"Read-only test automation"}).getByRole("button",{name:"Revoke",exact:true}).click();
  await expect(page.getByRole("row").filter({hasText:"Read-only test automation"})).toHaveCount(0);
  expect((await page.request.get("/api/v1/capabilities",{headers:{Authorization:"Bearer "+raw}})).status()).toBe(401);
  await page.getByRole("link",{name:"Notifications",exact:true}).click();
  await page.getByLabel("Enabled",{exact:true}).check();await page.getByRole("radio",{name:"Email",exact:true}).check();await expect(page.getByLabel("SMTP host",{exact:true})).toBeVisible();await page.getByRole("radio",{name:"Webhook",exact:true}).check();await expect(page.getByLabel("Webhook URL",{exact:true})).toBeVisible();
  await page.getByLabel("Enabled",{exact:true}).uncheck();
  await page.getByRole("link",{name:"Overview",exact:true}).click();await page.locator(".page-scroll").evaluate(el=>el.scrollTop=0);
  await page.screenshot({path:join(tmpdir(),"shipper-phase9-system.png"),fullPage:true});
  await page.setViewportSize({width:390,height:844});await expect(page.getByRole("heading",{name:"System",exact:true})).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:join(tmpdir(),"shipper-phase9-system-mobile.png"),fullPage:true});
  await page.request.post("http://127.0.0.1:9083/runtime-options",{data:{count:1}});await page.setViewportSize({width:1440,height:1000});await page.goto("/applications/application-web");await expect(page.getByRole("heading",{name:"Diagnostics",exact:true})).toBeVisible();await expect(page.getByRole("region", {name:"Application issue"})).toContainText("Routing unreachable");await page.locator(".page-scroll").evaluate(el=>el.scrollTop=0);await page.screenshot({path:join(tmpdir(),"shipper-phase9-diagnostics.png")});await page.getByRole("button", { name: "Inspect route", exact: true }).click();await expect(page.getByRole("region", { name: "Edit route" })).toBeVisible();expect(errors).toEqual([]);
});

test("Compose overview service logs survive inventory refreshes without replay",async({page})=>{
  await login(page);const original=await (await page.request.get("/api/applications/application-web")).json();
  const service={name:"api",image:"example/api:v1",port:3000,healthPath:"/",volumes:[],variables:[]};
  const release={...original.activeDeployment,id:"service-log-release",applicationId:"compose-browser",operation:"deploy",state:"succeeded",services:[service]};
  const application={...original.application,id:"compose-browser",name:"Service logs",sourceKind:"compose",method:"compose",activeMethod:"compose",latestDeployment:release,activeDeploymentId:release.id,config:{...original.application.config,method:"compose",composeSource:"paste"},containers:[{...original.application.containers[0],id:"service-container",service:"api",state:"running"}]};
  let revision=0,connections=0,stopped=false;
  await page.route("**/api/applications/compose-browser",route=>route.fulfill({json:{application:{...application,updatedAt:++revision,containers:application.containers.map(c=>({...c,state:stopped?"exited":"running"}))},deployments:[release],activeDeployment:release}}));
  await page.route("**/api/applications/compose-browser/diagnostics",route=>route.fulfill({json:{checks:[],observations:[],changesSinceServing:[],unknowns:[],durationSeconds:1,servingRelease:release,latestOperation:release}}));
  await page.route("**/api/applications/compose-browser/incidents",route=>route.fulfill({json:{items:[]}}));
  await page.route("**/api/applications/compose-browser/logs?**",route=>{connections++;return route.fulfill({contentType:"text/event-stream",body:'event: reset\ndata: {}\n\nevent: log\ndata: {"line":"Service is ready"}\n\nevent: end\ndata: {}\n\n'})});
  await page.goto("/applications/compose-browser");await page.getByRole("button",{name:"Open service api",exact:true}).click();await expect(page.getByRole("log",{name:"Runtime logs"})).toContainText("Service is ready");
  const before=connections;const session=await (await page.request.get("/api/auth/session")).json();
  for(let index=0;index<3;index++)await page.request.post("/api/refresh",{headers:{"X-CSRF-Token":session.csrfToken},data:{}});
  await expect.poll(()=>revision).toBeGreaterThan(1);await expect.poll(()=>connections).toBe(before);
  await page.getByRole("button",{name:"Pause runtime logs",exact:true}).click();await expect(page.getByRole("button",{name:"Resume runtime logs",exact:true})).toBeVisible();expect(connections).toBe(before);
  await page.getByRole("button",{name:"Resume runtime logs",exact:true}).click();expect(connections).toBe(before);
  await page.getByLabel("Search runtime logs").fill("ready");const download=page.waitForEvent("download");await page.getByRole("button",{name:"Download runtime logs",exact:true}).click();expect((await download).suggestedFilename()).toBe("runtime.log");
  await page.screenshot({path:join(tmpdir(),"shipper-phase9-service-logs.png")});
  stopped=true;
  await page.request.post("/api/refresh",{headers:{"X-CSRF-Token":session.csrfToken},data:{}});
  await expect(page.getByText("Log history",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Pause runtime logs",exact:true})).toHaveCount(0);
  await expect(page.getByRole("log",{name:"Runtime logs"})).toContainText("Service is ready");
});

test("Local folder source can be reviewed without connecting Git",async({page})=>{
 await login(page);await page.goto("/new");await page.getByRole("radio",{name:"Local files",exact:true}).check();
 const directory=await mkdtemp(join(tmpdir(),"shipper-folder-test-"));folders.push(directory);await writeFile(join(directory,"index.html"),"local folder fixture");await writeFile(join(directory,"compose.yaml"),"services:\n  web:\n    image: nginx:alpine\n    expose: ['80']\n");await writeFile(join(directory,".env"),"SECRET=must-not-upload");await writeFile(join(directory,".vpsenv"),"DUMMY=must-not-upload");await page.getByLabel("Project folder").setInputFiles(directory);
 await expect(page.locator(".source-file-list")).toContainText("index.html");await expect(page.locator(".source-file-list")).not.toContainText(".env");await expect(page.locator(".source-file-list")).not.toContainText(".vpsenv");await page.getByRole("button",{name:"Use these files",exact:true}).click();await expect(page.locator(".local-source-input>.mono")).toHaveText(/^[a-f0-9]{12}$/);await expect(page.getByRole("textbox",{name:"Repository URL",exact:true})).toHaveCount(0);await page.getByRole("button",{name:"Review with defaults",exact:true}).click();await expect(page.locator(".creation-review")).toContainText("Static site (automatic)");await expect(page.locator(".creation-review")).toContainText("Snapshot");await page.screenshot({path:join(tmpdir(),"shipper-phase95-local-review.png")});
 await page.getByRole("button",{name:"Edit configuration",exact:true}).click();
 await page.getByLabel("Build method",{exact:true}).selectOption("compose");
 await page.getByRole("button",{name:"Preview services",exact:true}).click();
 await expect(page.locator(".creation-services")).toContainText("nginx:alpine");
 await expect(page.getByLabel("Compose file",{exact:true})).toHaveValue("compose.yaml");
 await page.getByRole("button",{name:"Review deployment",exact:true}).click();
 await expect(page.locator(".creation-review")).toContainText("Docker Compose");
 await expect(page.locator(".creation-review")).toContainText("Snapshot");
 await page.getByRole("button",{name:"Edit source",exact:true}).click();
 await page.getByRole("radio",{name:"Git repository",exact:true}).check();
 const repository="https://github.com/example/compose-demo.git", commit="9".repeat(40);
 await page.route("**/api/repositories/inspect",route=>route.fulfill({json:{method:"dockerfile",commit:"8".repeat(40),variables:[]}}));
 await page.route("**/api/compose/preview",route=>{
   expect(route.request().postDataJSON().config).toMatchObject({repository,branch:"develop",method:"compose",composeSource:"git",composeFile:"deploy/compose.yaml"});
   return route.fulfill({json:{commit,composeFile:"deploy/compose.yaml",files:["deploy/compose.yaml"],services:[{name:"web",image:"nginx:alpine",port:80,volumes:[]}]}});
 });
 await page.getByLabel("Repository URL",{exact:true}).fill(repository);
 await page.getByLabel("Branch or commit",{exact:true}).fill("develop");
 await page.getByRole("button",{name:"Continue",exact:true}).click();
 await page.getByLabel("Build method",{exact:true}).selectOption("compose");
 await page.getByLabel("Compose file",{exact:true}).fill("deploy/compose.yaml");
 await page.getByRole("button",{name:"Review deployment",exact:true}).click();
 await expect(page.locator(".creation-review").getByRole("link", {name:"example/compose-demo",exact:true})).toHaveAttribute("href",repository);
 await expect(page.locator(".creation-review")).toContainText(commit.slice(0,12));
 await expect(page.locator(".creation-review")).toContainText("deploy/compose.yaml");
});
