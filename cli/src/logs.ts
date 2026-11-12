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
) {
  const app = await resolveApplication(client, reference);
  const controller = new AbortController();
  const stop = () => controller.abort();
  process.once("SIGINT", stop);
  const lines = options.lines ?? 100;
  const follow = Boolean(options.follow);
  let seen = 0;
  const stream = await client.stream(`/applications/${app.id}/logs?tail=${Math.max(1, options.tail ?? 200)}`, controller.signal);
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) break;
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
          process.stdout.write(colorLine(payload.line) + "\n");
          seen += 1;
          if (!follow && seen >= lines) {
            stop();
            return;
          }
        } else if (event === "failure") {
          throw new CliError((JSON.parse(data) as { message: string }).message);
        } else if (event === "end") {
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
    try { await reader.cancel(); } catch { /* the stream already ended */ }
  }
}
