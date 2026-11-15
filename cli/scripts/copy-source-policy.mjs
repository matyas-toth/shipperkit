import { copyFileSync } from "node:fs";

copyFileSync(new URL("../../internal/store/source-policy.json", import.meta.url), new URL("../dist/source-policy.json", import.meta.url));
