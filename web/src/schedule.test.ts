import { describe, expect, it } from "vitest";
import { parseSchedule, scheduleExpression } from "./schedule";
describe("schedule choices", () => {
  it.each([
    "",
    "*/15 * * * *",
    "30 */6 * * *",
    "*/7 * * * *",
    "12 * * * *",
    "30 3 * * *",
    "30 9 * * 1,3,5",
    "0 1 31 * *",
    "0 0 1 * 1",
    "0 0 * 2-8/2 1-5",
  ])("preserves %s", (value) => {
    expect(scheduleExpression(parseSchedule(value))).toBe(value);
  });
  it("keeps unusual patterns in manual mode without changing their semantics", () => {
    expect(parseSchedule("0 0 1 * 1").mode).toBe("custom");
    expect(parseSchedule("0 0 * * 7").mode).toBe("custom");
  });
});
