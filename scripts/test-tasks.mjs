import { spawnSync } from "node:child_process";
import { goCommand } from "./go.mjs";
const command=goCommand();
let result=spawnSync(command,["generate","./internal/startup"],{stdio:"inherit",windowsHide:true});
if(result.status===0)result=spawnSync(command,["test","./internal/deploy","-run","TestDockerLifecycleTasks","-v","-count=1","-timeout=10m"],{env:{...process.env,SHIPPER_TEST_TASKS:"1"},stdio:"inherit",windowsHide:true});
process.exitCode=result.status??1;
