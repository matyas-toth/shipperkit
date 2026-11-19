import { describe, expect, it } from "vitest";
import { emptyVariable, formatEnvironment, looksLikeEnvironment, mergeEnvironment, parseEnvironment, parseEnvironmentFile, retainedValue, variableSignature, variablesForSave } from "./env";
import type { Variable } from "./types";

describe("simple environment editing", () => {
  it("pastes complete files locally with secret build/runtime defaults", () => {
    const items = parseEnvironment("# comment\r\nexport TOKEN='hello world'\r\nURL=https://example.com/#hash\r\nEMPTY=\r\nEXPRESSION=$(not-executed)");
    expect(items.map(v => v.value)).toEqual(["hello world", "https://example.com/#hash", "", "$(not-executed)"]);
    expect(items.every(v => v.secret && v.scope === "both")).toBe(true);
    expect(looksLikeEnvironment("TOKEN=value")).toBe(true);expect(looksLikeEnvironment("https://example.com/?token=value")).toBe(false);
    expect(() => parseEnvironment("A=1\nA=2")).toThrow("appears more than once");
  });
  it("preserves advanced settings when paste replaces a saved value", () => {
    const old: Variable = { name:"TOKEN",value:"",secret:false,scope:"build",keep:true,hasValue:true };
    const result = mergeEnvironment([old,emptyVariable()],parseEnvironment("TOKEN=replaced\nNEW=value"));
    expect(result[0]).toMatchObject({name:"TOKEN",value:"replaced",secret:false,scope:"build",keep:false});
    expect(result[1]).toMatchObject({name:"NEW",scope:"both",secret:true});
    expect(old.keep).toBe(true);
  });
  it("ignores empty rows and reveal-only changes while retaining deliberate replacements", () => {
    const old: Variable = {name:"TOKEN",value:"",keep:true,secret:true,scope:"runtime"};
    expect(variableSignature([old,emptyVariable()])).toBe(variableSignature([old]));
    expect(variableSignature([{...old,value:"revealed"}])).toBe(variableSignature([old]));
    expect(variableSignature([{...old,keep:false,value:""}])).not.toBe(variableSignature([old]));
    expect(variablesForSave([{...emptyVariable(),name:"EMPTY"},emptyVariable()])).toHaveLength(1);
  });
  it("round-trips the .env editor without losing retained secrets or settings", () => {
    const saved: Variable[] = [
      { name: "TOKEN", value: "", secret: true, scope: "runtime", keep: true, hasValue: true },
      { name: "GREETING", value: "hello world", secret: false, scope: "build" },
      { name: "MULTI", value: "a\nb \"c\"", secret: false, scope: "both" },
    ];
    const text = formatEnvironment(saved);
    expect(text).toBe(["TOKEN=" + retainedValue, 'GREETING="hello world"', 'MULTI="a\\nb \\"c\\""'].join("\n"));
    expect(variableSignature(parseEnvironmentFile(text, saved))).toBe(variableSignature(saved));
    const edited = parseEnvironmentFile(text + "\n# note\nNEW=value", saved);
    expect(edited[3]).toMatchObject({ name: "NEW", value: "value", scope: "both" });
    expect(parseEnvironmentFile("GREETING=changed", saved)[0]).toMatchObject({ value: "changed", scope: "build", secret: false });
    expect(() => parseEnvironmentFile("OTHER=" + retainedValue, saved)).toThrow("Enter a value for OTHER");
    expect(() => parseEnvironmentFile('A="open', saved)).toThrow("unclosed quote");
  });
});
