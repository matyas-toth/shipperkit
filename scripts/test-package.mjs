// Runs an already-built shipper:local image using isolated, explicitly owned data.
import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { createServer } from "node:net";
import { statSync } from "node:fs";
import { request as httpRequest } from "node:http";

function docker(...args) {
  const result = spawnSync("docker", args, { encoding: "utf8", windowsHide: true });
  if (result.status !== 0) throw Error(result.stderr || "Docker command failed");return result.stdout.trim();
}
async function freePort() { const server = createServer();await new Promise(resolve => server.listen(0,"127.0.0.1",resolve));const port = server.address().port;await new Promise(resolve => server.close(resolve));return port; }
async function poll(fn, timeout = 120000) { const deadline = Date.now() + timeout;let last;while (Date.now() < deadline) { try { const value = await fn();if (value) return value; }catch (e) { last = e; }await new Promise(resolve => setTimeout(resolve,200)); }throw last || Error("Timed out waiting for the packaged application"); }
const id = randomBytes(8).toString("hex"), name = "shipper-package-" + id, volume = name + "-data", ports = await Promise.all([freePort(),freePort(),freePort()]);
const origin = "http://127.0.0.1:" + ports[0], setupToken = randomBytes(32).toString("hex"), password = randomBytes(24).toString("hex");
let cookie = "", csrf = "", instance = "", application;
async function api(path, body) {
  const response = await fetch(origin + "/api" + path, { method: body === undefined ? "GET" : "POST", headers: { Cookie:cookie, ...(body === undefined ? {} : { "Content-Type":"application/json", "X-CSRF-Token":csrf }) }, body:body === undefined ? undefined : JSON.stringify(body), signal:AbortSignal.timeout(10000) });
  if (response.headers.get("set-cookie")) cookie = response.headers.get("set-cookie").split(";")[0];
  const value = await response.json();if (!response.ok) throw Object.assign(Error(value.error?.message || "API request failed"), {status:response.status});return value;
}
async function waitDeployment(job) { return poll(async () => { const detail = await api("/applications/" + job.applicationId);const d = detail.deployments.find(d => d.id === job.id);return d && ["succeeded","failed"].includes(d.state) ? d : undefined; }); }
function siteRequest(domain) { return new Promise((resolve,reject) => { const request=httpRequest({hostname:"127.0.0.1",port:ports[1],path:"/",headers:{Host:domain},timeout:10000},response=>{let body="";response.setEncoding("utf8");response.on("data",chunk=>body+=chunk);response.on("end",()=>resolve({status:response.statusCode,release:response.headers["x-shipper-deployment"],body}));});request.on("error",reject);request.on("timeout",()=>request.destroy(Error("Site request timed out")));request.end(); }); }
function removeContainer(containerName, label, expected) { try { const labels = JSON.parse(docker("inspect","--format","{{json .Config.Labels}}",containerName));if (labels[label] === expected) docker("rm","-f",containerName); }catch {} }
function removeVolume(volumeName,label,expected) { try { const labels = JSON.parse(docker("volume","inspect","--format","{{json .Labels}}",volumeName));if (labels[label] === expected) docker("volume","rm",volumeName); }catch {} }
try {
  docker("volume","create","--label","io.shipper.test-run=" + id,volume);
  const group = process.env.DOCKER_GID || (process.platform === "linux" ? String(statSync("/var/run/docker.sock").gid) : "0");
  docker("run","-d","--name",name,"--label","io.shipper.test-run="+id,"--label","io.shipper.control-plane=true","--read-only","--tmpfs","/tmp:size=16m,mode=1777","--cap-drop","ALL","--security-opt","no-new-privileges:true","--group-add",group,"-p","127.0.0.1:"+ports[0]+":8080","-v",volume+":/data","-v","/var/run/docker.sock:/var/run/docker.sock:ro","-e","SHIPPER_SETUP_TOKEN="+setupToken,"-e","SHIPPER_HTTP_PORT="+ports[1],"-e","SHIPPER_HTTPS_PORT="+ports[2],"-e","SHIPPER_PROXY_BIND=127.0.0.1",process.env.SHIPPER_TEST_IMAGE || "shipper:local");
  await poll(() => fetch(origin+"/api/health").then(r=>r.ok));
  const session = await api("/auth/setup",{email:"package@example.test",password,setupToken});csrf=session.csrfToken;
  instance = (await api("/system")).instanceId;if (!/^[a-f0-9]{64}$/.test(instance)) throw Error("The packaged instance did not expose its stable identity");
  if (docker("exec",name,"id","-u") !== "65532") throw Error("The control plane is not running without root");
  const job = await api("/applications",{name:"Packaged website",config:{repository:"https://github.com/mdn/beginner-html-site.git",publishDirectory:".",routing:"files",domain:"",https:false,htaccess:false,branch:""}});
  application = job.applicationId;const first = await waitDeployment(job);if (first.state !== "succeeded") throw Error(first.error);
  if ((await api("/applications/"+application+"/domain-check",{})).state !== "verified") throw Error("Domain verification failed from inside the packaged control plane");
  const site = await siteRequest(first.config.domain);if (site.status!==200||site.release!==first.id) throw Error(`Traefik did not serve the packaged deployment: ${site.status} ${site.release} ${site.body.slice(0,160)}`);
  docker("restart",name);await poll(() => api("/auth/session").then(s => !!s.user));
  const current = await api("/applications/"+application);if (current.application.activeDeploymentId!==first.id) throw Error("The active deployment did not survive control-plane restart");
  const bad = await api("/applications/"+application+"/deployments",{config:{...first.config,publishDirectory:"does-not-exist"}});const failure=await waitDeployment(bad);if(failure.state!=="failed") throw Error("A broken deployment unexpectedly succeeded");
  const after = await siteRequest(first.config.domain);if(after.status!==200||after.release!==first.id) throw Error("A failed deployment disturbed the active site");
  const deletion=await api("/applications/"+application+"/delete",{confirmName:"Packaged website"});await poll(async()=>{const logs=await api(`/applications/${application}/deployments/${deletion.id}/logs`);if(logs.state==="failed")throw Error("Packaged deletion failed");return logs.state==="succeeded";});
  docker("exec",name,"shipper","healthcheck");
  console.log("Passed packaged non-root setup, public Git deployment, Apache/Traefik response, in-container domain verification, restart/session persistence, failed-release preservation, and deletion.");
} finally {
  if (instance) {
    const containers = docker("ps","-aq","--filter","label=io.shipper.instance="+instance).split(/\s+/).filter(Boolean);
    for (const container of containers) removeContainer(container,"io.shipper.instance",instance);
    const prefix="shipper-"+instance.slice(0,12);
    try {const labels=JSON.parse(docker("network","inspect","--format","{{json .Labels}}",prefix+"-edge"));if(labels["io.shipper.instance"]===instance)docker("network","rm",prefix+"-edge");}catch{}
    removeVolume(prefix+"-proxy-data","io.shipper.instance",instance);
    const images=docker("image","ls","-q","--filter","label=io.shipper.instance="+instance).split(/\s+/).filter(Boolean);for(const image of new Set(images)){try{const labels=JSON.parse(docker("image","inspect","--format","{{json .Config.Labels}}",image));if(labels["io.shipper.instance"]===instance)docker("image","rm",image);}catch{}}
  }
  removeContainer(name,"io.shipper.test-run",id);removeVolume(volume,"io.shipper.test-run",id);
}
