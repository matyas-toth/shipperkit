import { describe,it,expect } from "vitest";
import { excludedSource,sourceArchive } from "./local-source";
describe("local upload boundary",()=>{
 it("keeps source templates while excluding credentials and dependencies",()=>{for(const name of [".env","a/.env.production",".ssh/id_ed25519","cert.pem","node_modules/a.js",".git/config"])expect(excludedSource(name)).toBe(true);for(const name of ["src/app.ts",".env.example","Dockerfile","compose.yaml"])expect(excludedSource(name)).toBe(false)});
 it("creates a portable tar with bounded regular files",async()=>{const blob=sourceArchive([{name:"index.html",file:new File(["hello"],"index.html")}]);const data=new Uint8Array(await blob.arrayBuffer());expect(new TextDecoder().decode(data.slice(0,10))).toBe("index.html");expect(new TextDecoder().decode(data.slice(512,517))).toBe("hello");expect(blob.size%512).toBe(0);expect(()=>sourceArchive([{name:"../outside",file:new File(["x"],"outside")}])).toThrow()});
});
