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
function siteRequest(domain, path = "/") { return new Promise((resolve,reject) => { const request=httpRequest({hostname:"127.0.0.1",port:ports[1],path,headers:{Host:domain},timeout:10000},response=>{let body="";response.setEncoding("utf8");response.on("data",chunk=>body+=chunk);response.on("end",()=>resolve({status:response.statusCode,release:response.headers["x-shipper-deployment"],body}));});request.on("error",reject);request.on("timeout",()=>request.destroy(Error("Site request timed out")));request.end(); }); }
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
  const branches = await api("/repositories/branches",{repository:"https://github.com/mdn/beginner-html-site.git"});if(!branches.items.length || !branches.default || !branches.items.includes(branches.default))throw Error("Public branch selection failed");
  const job = await api("/applications",{name:"Packaged website",config:{repository:"https://github.com/mdn/beginner-html-site.git",publishDirectory:".",routing:"files",domain:"",https:false,htaccess:false,branch:""}});
  application = job.applicationId;let first = await waitDeployment(job);if (first.state !== "succeeded") throw Error(first.error);
  if ((await api("/applications/"+application+"/domain-check",{})).state !== "verified") throw Error("Domain verification failed from inside the packaged control plane");
  const site = await siteRequest(first.config.domain);if (site.status!==200||site.release!==first.id) throw Error(`Traefik did not serve the packaged deployment: ${site.status} ${site.release} ${site.body.slice(0,160)}`);
  first.config.branch=first.commit;
  const buildSecret = randomBytes(24).toString("hex");
  const builtJob = await api("/applications/"+application+"/deployments",{config:{...first.config,method:"static",installCommand:"node --version",buildCommand:`node -e "if (!process.env.BUILD_TOKEN) process.exit(1);require('fs').appendFileSync('index.html', process.env.BUILD_MARKER);require('fs').writeFileSync('private.key','excluded-build-artifact')"`,healthPath:"/",variables:[{name:"BUILD_TOKEN",value:buildSecret,secret:true,scope:"build"},{name:"BUILD_MARKER",value:"packaged-build-output",secret:false,scope:"build"}]}});
  if (builtJob.config.variables[0].value !== "" || !builtJob.config.variables[0].keep) throw Error("The deployment response exposed a build secret");
  first = await waitDeployment(builtJob);if(first.state!=="succeeded") {const logs=await api(`/applications/${application}/deployments/${first.id}/logs`);throw Error(first.error+"\n"+logs.items.map(l=>l.line).join("\n"));}
  if (!(await siteRequest(first.config.domain)).body.includes("packaged-build-output")) throw Error("The packaged BuildKit client did not build the static output");
  for(const path of ["/.shipper-httpd.conf","/private.key"])if((await siteRequest(first.config.domain,path)).status===200)throw Error("A private build artifact was published");
  const routeJob = await api("/applications/"+application+"/domains",{domains:[{id:"primary",hostname:first.config.domain,https:false,primary:true,redirect:false},{id:"alias",hostname:"alias-"+first.config.domain,https:false,primary:false,redirect:false}]});
  if ((await waitDeployment(routeJob)).state!=="succeeded" || (await siteRequest("alias-"+first.config.domain)).release!==first.id) throw Error("Packaged route-only update failed");
  const stopped = await api("/applications/"+application+"/lifecycle",{action:"stop"});if((await waitDeployment(stopped)).state!=="succeeded")throw Error("Packaged stop failed");
  const started = await api("/applications/"+application+"/lifecycle",{action:"start"});if((await waitDeployment(started)).state!=="succeeded")throw Error("Packaged start failed");
  docker("restart",name);await poll(() => api("/auth/session").then(s => !!s.user));
  const current = await api("/applications/"+application);if (current.application.activeDeploymentId!==first.id) throw Error("The active deployment did not survive control-plane restart");
  const bad = await api("/applications/"+application+"/deployments",{config:{...first.config,publishDirectory:"does-not-exist"}});const failure=await waitDeployment(bad);if(failure.state!=="failed") throw Error("A broken deployment unexpectedly succeeded");
  const after = await siteRequest(first.config.domain);if(after.status!==200||after.release!==first.id) throw Error("A failed deployment disturbed the active site");
  const deletion=await api("/applications/"+application+"/delete",{confirmName:"Packaged website"});await poll(async()=>{const logs=await api(`/applications/${application}/deployments/${deletion.id}/logs`);if(logs.state==="failed")throw Error("Packaged deletion failed");return logs.state==="succeeded";});

  const composePassword=randomBytes(24).toString("hex"), composeHost="compose-"+id+".localhost";
  const composeYAML=`services:
  web:
    build:
      context: .
      dockerfile_inline: |
        FROM httpd:2.4-alpine
    expose: ['80']
    configs:
      - source: page
        target: /usr/local/apache2/htdocs/index.html
    depends_on:
      database: {condition: service_healthy}
  database:
    image: postgres:17-alpine
    environment:
      POSTGRES_PASSWORD: \${DATABASE_PASSWORD:?required}
    volumes: ['database:/var/lib/postgresql/data']
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U postgres']
      interval: 2s
      timeout: 2s
      retries: 20
configs:
  page:
    content: Packaged Compose works
volumes:
  database:
`;
  const composeConfig={method:"compose",composeSource:"paste",composeYAML,rootDirectory:".",hooks:[{stage:"before_deploy",command:{service:"web",argv:["test","-s","/usr/local/apache2/htdocs/index.html"],mode:"job",timeout:30}},{stage:"before_start",command:{service:"web",argv:["test","-s","/usr/local/apache2/htdocs/index.html"],mode:"job",timeout:30}}],variables:[{name:"DATABASE_PASSWORD",value:composePassword,secret:true,scope:"both"}],domains:[{id:"web",service:"web",hostname:composeHost,https:false,primary:true,redirect:false}]};
  const preview=await api("/compose/preview",{config:composeConfig});if(preview.services.length!==2)throw Error("Packaged Compose preview failed");
  const composeJob=await api("/applications",{name:"Packaged Compose",config:composeConfig});const composeRelease=await waitDeployment(composeJob);
  if(composeRelease.state!=="succeeded"){const logs=await api(`/applications/${composeJob.applicationId}/deployments/${composeJob.id}/logs`);throw Error(composeRelease.error+" "+JSON.stringify(logs));}
  const composeSite=await siteRequest(composeHost);if(composeSite.status!==200||!composeSite.body.includes("Packaged Compose works"))throw Error("Packaged Compose build/config mount failed");
  const composeDetail=await api("/applications/"+composeJob.applicationId);if(composeDetail.application.config.composeYAML||JSON.stringify(composeDetail).includes(composePassword))throw Error("Packaged Compose disclosed a secret");
  if((await api(`/applications/${composeJob.applicationId}/variables/reveal`,{service:"database",name:"POSTGRES_PASSWORD"})).value!==composePassword)throw Error("Inherited Compose value reveal failed");
  const savedTask=await api(`/applications/${composeJob.applicationId}/tasks`,{name:"Print page",enabled:true,command:{service:"web",argv:["cat","/usr/local/apache2/htdocs/index.html"],mode:"job",timeout:30}});
  const taskJob=await api(`/applications/${composeJob.applicationId}/tasks/${savedTask.id}/run`,{});if((await waitDeployment(taskJob)).state!=="succeeded")throw Error("Packaged saved task failed");
  const history=await api(`/applications/${composeJob.applicationId}/tasks`);if(history.runs.filter(run=>run.state==="succeeded").length<2)throw Error("Packaged task attempts were not recorded");
  const web=await poll(async()=>{const detail=await api(`/applications/${composeJob.applicationId}`);return detail.application.containers.find(c=>c.service==="web");});
  if(!docker("logs",web.id).includes("[Shipper] Before start"))throw Error("Packaged embedded startup helper was not executed");
  docker("restart",name);await poll(()=>api("/auth/session").then(s=>!!s.user));
  const composeUpdate=await api(`/applications/${composeJob.applicationId}/deployments`,{config:composeDetail.application.config});if((await waitDeployment(composeUpdate)).state!=="succeeded")throw Error("Packaged retained Compose redeploy failed");
  if((await siteRequest(composeHost)).status!==200)throw Error("Packaged Compose restart/redeploy lost routing");
  const composeDelete=await api(`/applications/${composeJob.applicationId}/delete`,{confirmName:"Packaged Compose"});await poll(async()=>{const logs=await api(`/applications/${composeJob.applicationId}/deployments/${composeDelete.id}/logs`);if(logs.state==="failed")throw Error("Packaged Compose deletion failed");return logs.state==="succeeded";});
  docker("exec",name,"shipper","healthcheck");
  console.log("Passed packaged non-root setup, public Git, BuildKit static build with secret mounts, route-only domains, lifecycle, domain verification, restart/session persistence, failed-release preservation, Compose builds/config mounts, inherited variables, first-deploy tasks, embedded startup helpers, saved job history, retained redeployment, and deletion.");
} finally {
  if (instance) {
    const containers = docker("ps","-aq","--filter","label=io.shipper.instance="+instance).split(/\s+/).filter(Boolean);
    for (const container of containers) removeContainer(container,"io.shipper.instance",instance);
    const prefix="shipper-"+instance.slice(0,12);
    const ownedNetworks=docker("network","ls","-q","--filter","label=io.shipper.instance="+instance).split(/\s+/).filter(Boolean);for(const network of ownedNetworks){try{const labels=JSON.parse(docker("network","inspect","--format","{{json .Labels}}",network));if(labels["io.shipper.instance"]===instance)docker("network","rm",network);}catch{}}
    const ownedVolumes=docker("volume","ls","-q","--filter","label=io.shipper.instance="+instance).split(/\s+/).filter(Boolean);for(const ownedVolume of ownedVolumes)removeVolume(ownedVolume,"io.shipper.instance",instance);
    const images=docker("image","ls","-q","--filter","label=io.shipper.instance="+instance).split(/\s+/).filter(Boolean);for(const image of new Set(images)){try{const labels=JSON.parse(docker("image","inspect","--format","{{json .Config.Labels}}",image));if(labels["io.shipper.instance"]===instance)docker("image","rm",image);}catch{}}
  }
  removeContainer(name,"io.shipper.test-run",id);removeVolume(volume,"io.shipper.test-run",id);
}
