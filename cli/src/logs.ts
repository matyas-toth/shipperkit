import { CliError, type Client } from "./client";
import { resolveApplication } from "./apps";
import { paint } from "./output";

function colorLine(line: string) {
  const colors = paint();
  if (/\b(error|failed|fatal|exception)\b/i.test(line)) return colors.red(line);
  if (/\b(warn|warning|deprecated)\b/i.test(line)) return colors.yellow(line);
  return line.replace(/^(\S+\s+\S+)\s/, match => colors.dim(match));
}

export async function logs(
  client: Client,
  reference: string,
  options: { tail?: number; lines?: number; follow?: boolean },
  json = false,
  write: (value: string) => void = value => { process.stdout.write(value); },
) {
  const lines = options.lines ?? options.tail ?? 100;
  const follow = Boolean(options.follow);
  const tail = follow ? options.tail ?? lines : lines;
  if (![lines, options.tail ?? lines].every(value => Number.isInteger(value) && value >= 1 && value <= 500)) throw new CliError("Choose between 1 and 500 log lines");
  const app = await resolveApplication(client, reference);
  const controller = new AbortController();
  const stop = () => controller.abort();
  process.once("SIGINT", stop);
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  const history: string[] = [];
  let complete = false;
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    const stream = await client.stream(`/applications/${app.id}/logs?tail=${tail}&follow=${follow}`, controller.signal);
    reader = stream.getReader();
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) { complete = true; break; }
      buffer += decoder.decode(chunk.value, { stream: true });
      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        const block = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        let event = "message";
        let data = "";
        for (const line of block.split("\n")) {
          if (line.startsWith("event:")) event = line.slice(6).trim();
          else if (line.startsWith("data:")) data += line.slice(5).trim();
        }
        if (event === "log") {
          const payload = JSON.parse(data) as { line: string };
          if (json && follow) write(JSON.stringify({ line: payload.line }) + "\n");
          else if (json) history.push(payload.line);
          else write(colorLine(payload.line) + "\n");
        } else if (event === "failure") {
          throw new CliError((JSON.parse(data) as { message: string }).message);
        } else if (event === "end") {
          complete = true;
          return;
        }
        boundary = buffer.indexOf("\n\n");
      }
    }
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") return;
    throw error;
  } finally {
    process.off("SIGINT", stop);
    controller.abort();
    try { await reader?.cancel(); } catch { /* the stream already ended */ }
    reader?.releaseLock();
    if (json && !follow && complete) write(JSON.stringify({ lines: history }, null, 2) + "\n");
  }
}
