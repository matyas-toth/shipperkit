export interface ScheduleChoice {
  mode:
    | "manual"
    | "minutes"
    | "hourly"
    | "hours"
    | "daily"
    | "weekly"
    | "monthly"
    | "custom";
  interval: number;
  minute: number;
  hour: number;
  day: number;
  days: number[];
  custom: string;
}
export function parseSchedule(value: string): ScheduleChoice {
  const c: ScheduleChoice = {
    mode: "custom",
    interval: 15,
    minute: 0,
    hour: 3,
    day: 1,
    days: [1],
    custom: value,
  };
  if (!value) return { ...c, mode: "manual" };
  let m = value.match(/^\*\/(\d+) \* \* \* \*$/);
  if (m && +m[1] >= 1 && +m[1] <= 59 && 60 % +m[1] === 0)
    return { ...c, mode: "minutes", interval: +m[1] };
  m = value.match(/^(\d+) \*\/(\d+) \* \* \*$/);
  if (m && +m[1] < 60 && +m[2] >= 1 && +m[2] < 24 && 24 % +m[2] === 0)
    return { ...c, mode: "hours", minute: +m[1], interval: +m[2] };
  m = value.match(/^(\d+) \* \* \* \*$/);
  if (m && +m[1] < 60) return { ...c, mode: "hourly", minute: +m[1] };
  m = value.match(/^(\d+) (\d+) (\*|\d+) \* (\*|[0-6](?:,[0-6])*)$/);
  if (!m || +m[1] > 59 || +m[2] > 23) return c;
  if (m[3] === "*" && m[4] === "*")
    return { ...c, mode: "daily", minute: +m[1], hour: +m[2] };
  if (m[3] === "*" && m[4] !== "*")
    return {
      ...c,
      mode: "weekly",
      minute: +m[1],
      hour: +m[2],
      days: m[4].split(",").map(Number),
    };
  if (m[4] === "*" && +m[3] >= 1 && +m[3] <= 31)
    return { ...c, mode: "monthly", minute: +m[1], hour: +m[2], day: +m[3] };
  return c;
}
export function scheduleExpression(c: ScheduleChoice): string {
  switch (c.mode) {
    case "manual":
      return "";
    case "minutes":
      return `*/${c.interval} * * * *`;
    case "hours":
      return `${c.minute} */${c.interval} * * *`;
    case "hourly":
      return `${c.minute} * * * *`;
    case "daily":
      return `${c.minute} ${c.hour} * * *`;
    case "weekly":
      return `${c.minute} ${c.hour} * * ${[...c.days].sort().join(",")}`;
    case "monthly":
      return `${c.minute} ${c.hour} ${c.day} * *`;
    case "custom":
      return c.custom;
  }
}

export function describeSchedule(expression: string): string {
  const c = parseSchedule(expression);
  const minute = String(c.minute).padStart(2, "0");
  const time = String(c.hour).padStart(2, "0") + ":" + minute;
  switch (c.mode) {
    case "manual": return "Manual";
    case "minutes": return c.interval === 1 ? "Every minute" : "Every " + c.interval + " minutes";
    case "hourly": return "Hourly at :" + minute;
    case "hours": return c.interval === 1 ? "Hourly at :" + minute : "Every " + c.interval + " hours at :" + minute;
    case "daily": return "Daily at " + time;
    case "weekly": return [1, 2, 3, 4, 5, 6, 0].filter(day => c.days.includes(day))
      .map(day => ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][day]).join(", ") + " at " + time;
    case "monthly": return "Monthly on day " + c.day + " at " + time;
    case "custom": return "Custom cron";
  }
}
