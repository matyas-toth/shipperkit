import {
  For,
  Show,
  createEffect,
  createSignal,
  createUniqueId,
  onCleanup,
} from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import {
  parseSchedule,
  scheduleExpression,
  type ScheduleChoice,
} from "../schedule";

export default function SchedulePicker(props: {
  value: string;
  timezone: string;
  change: (schedule: string, timezone: string) => void;
}) {
  const auth = useAuth(),
    id = createUniqueId(),
    initial = parseSchedule(props.value);
  const [choice, setChoice] = createSignal<ScheduleChoice>(initial),
    [next, setNext] = createSignal<number[]>([]),
    [error, setError] = createSignal("");
  let last = props.value;
  createEffect(() => {
    if (props.value !== last) {
      last = props.value;
      setChoice(parseSchedule(props.value));
    }
  });
  function update(value: Partial<ScheduleChoice>) {
    const c = { ...choice(), ...value };
    if (value.mode === "custom") c.custom = props.value;
    if (value.mode === "hours" && ![1, 2, 3, 4, 6, 8, 12].includes(c.interval))
      c.interval = 2;
    if (
      value.mode === "minutes" &&
      ![1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30].includes(c.interval)
    )
      c.interval = 15;
    setChoice(c);
    const expression = scheduleExpression(c);
    last = expression;
    props.change(expression, props.timezone);
  }
  createEffect(() => {
    const schedule = props.value,
      timezone = props.timezone;
    let cancelled = false;
    setNext([]);
    setError("");
    const timer = setTimeout(() => {
      if (schedule)
        void post<{ items: number[] }>(
          "/protection/schedule-preview",
          { schedule, timezone },
          auth.session()?.csrfToken,
        )
          .then((v) => {
            if (!cancelled) setNext(v.items);
          })
          .catch((e) => {
            if (!cancelled) setError(e.message);
          });
    }, 300);
    onCleanup(() => {
      cancelled = true;
      clearTimeout(timer);
    });
  });
  const time = () =>
    `${String(choice().hour).padStart(2, "0")}:${String(choice().minute).padStart(2, "0")}`;
  const manual = () => choice().mode === "custom";
  const guided = () => {
    const candidate = parseSchedule(props.value);
    return candidate.mode !== "manual" && candidate.mode !== "custom" && scheduleExpression(candidate) === props.value ? candidate : undefined;
  };
  return (
    <div class="schedule-picker">
      <div class="form-fields">
        <label for={id + "-mode"}>Repeat</label>
        <select
          id={id + "-mode"}
          class="input"
          value={choice().mode}
          onChange={(e) =>
            update({ mode: e.currentTarget.value as ScheduleChoice["mode"] })
          }
        >
          <option value="manual">Manually</option>
          <option value="minutes" disabled={manual()}>Every few minutes</option>
          <option value="hourly" disabled={manual()}>Every hour</option>
          <option value="hours" disabled={manual()}>Every few hours</option>
          <option value="daily" disabled={manual()}>Every day</option>
          <option value="weekly" disabled={manual()}>Every week</option>
          <option value="monthly" disabled={manual()}>Every month</option>
          <option value="custom">Custom cron</option>
        </select>
        <Show when={choice().mode === "minutes" || choice().mode === "hours"}>
          <label for={id + "-interval"}>
            {choice().mode === "minutes" ? "Minutes" : "Hours"}
          </label>
          <select
            id={id + "-interval"}
            class="input"
            value={choice().interval}
            onChange={(e) =>
              update({ interval: Number(e.currentTarget.value) })
            }
          >
            <For
              each={
                choice().mode === "minutes"
                  ? [1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30]
                  : [1, 2, 3, 4, 6, 8, 12]
              }
            >
              {(n) => <option value={n}>{n}</option>}
            </For>
          </select>
        </Show>
        <Show when={choice().mode === "hourly" || choice().mode === "hours"}>
          <label for={id + "-minute"}>At minute</label>
          <input
            id={id + "-minute"}
            class="input"
            type="number"
            min="0"
            max="59"
            value={choice().minute}
            onInput={(e) => update({ minute: Number(e.currentTarget.value) })}
          />
        </Show>
        <Show when={["daily", "weekly", "monthly"].includes(choice().mode)}>
          <label for={id + "-time"}>At</label>
          <input
            id={id + "-time"}
            type="time"
            class="input"
            required
            value={time()}
            onInput={(e) => {
              const [hour, minute] = e.currentTarget.value
                .split(":")
                .map(Number);
              update({ hour, minute });
            }}
          />
        </Show>
        <Show when={choice().mode === "weekly"}>
          <label>On</label>
          <div class="schedule-days" role="group" aria-label="Days of the week">
            <For each={[1, 2, 3, 4, 5, 6, 0]}>
              {(day) => (
                <button
                  type="button"
                  aria-pressed={choice().days.includes(day)}
                  disabled={choice().days.length === 1 && choice().days.includes(day)}
                  classList={{ selected: choice().days.includes(day) }}
                  onClick={() =>
                    update({
                      days: choice().days.includes(day)
                        ? choice().days.filter((v) => v !== day)
                        : [...choice().days, day],
                    })
                  }
                >
                  {["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][day]}
                </button>
              )}
            </For>
          </div>
        </Show>
        <Show when={choice().mode === "monthly"}>
          <label for={id + "-day"}>Day</label>
          <input
            id={id + "-day"}
            class="input"
            type="number"
            min="1"
            max="31"
            value={choice().day}
            onInput={(e) => update({ day: Number(e.currentTarget.value) })}
          />
        </Show>
        <Show when={props.value || choice().mode === "custom"}>
          <label for={id + "-cron"}>Cron</label>
          <input
            id={id + "-cron"}
            class="input mono"
            aria-label="Cron expression"
            readOnly={choice().mode !== "custom"}
            value={props.value}
            onInput={(e) => {
              last = e.currentTarget.value;
              setChoice({ ...choice(), custom: last });
              props.change(last, props.timezone);
            }}
          />
          <label for={id + "-zone"}>Timezone</label>
          <input
            id={id + "-zone"}
            class="input"
            required
            value={props.timezone}
            onInput={(e) => props.change(props.value, e.currentTarget.value)}
          />
          <Show when={manual() && guided()}>{candidate =>
            <Button variant="ghost" class="use-guided" onClick={() => setChoice(candidate())}>Use guided controls</Button>
          }</Show>
        </Show>
      </div>
      <Show when={error()}>
        <ErrorNotice message={error()} />
      </Show>
      <Show when={next().length}>
        <ol class="schedule-next" aria-label="Next three runs">
          <For each={next()}>
            {(n) => (
              <li>
                <time datetime={new Date(n * 1000).toISOString()}>
                  {new Date(n * 1000).toLocaleString(undefined, {
                    timeZone: props.timezone,
                    timeZoneName: "short",
                  })}
                </time>
              </li>
            )}
          </For>
        </ol>
      </Show>
    </div>
  );
}
