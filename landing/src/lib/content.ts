import { getCollection, type CollectionEntry } from "astro:content";

/* One place that decides what is published and in which order, so pages,
   feeds, the sitemap and llms.txt always agree. */

const visible = (entry: { data: { draft: boolean } }) => import.meta.env.DEV || !entry.data.draft;

export const SITE = "https://shipperkit.dev";

export async function posts(): Promise<CollectionEntry<"blog">[]> {
  const all = await getCollection("blog", visible);
  return all.sort((a, b) => b.data.date.getTime() - a.data.date.getTime() || a.data.title.localeCompare(b.data.title));
}

export async function comparisons(): Promise<CollectionEntry<"compare">[]> {
  const all = await getCollection("compare", visible);
  return all.sort((a, b) => a.data.order - b.data.order);
}

/** Orders 1.0.0-rc.9 before 1.0.0-rc.10 and both before 1.0.0. */
export function compareVersions(a: string, b: string): number {
  const [coreA, preA] = a.split("-", 2);
  const [coreB, preB] = b.split("-", 2);
  const nums = (v: string) => v.split(".").map(Number);
  const [x, y] = [nums(coreA), nums(coreB)];
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] - y[i];
  if (!preA || !preB) return preA ? -1 : preB ? 1 : 0;
  const [pa, pb] = [preA.split("."), preB.split(".")];
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    if (pa[i] === undefined) return -1;
    if (pb[i] === undefined) return 1;
    const [na, nb] = [Number(pa[i]), Number(pb[i])];
    const diff = Number.isNaN(na) || Number.isNaN(nb) ? pa[i].localeCompare(pb[i]) : na - nb;
    if (diff) return diff;
  }
  return 0;
}

export async function releases(): Promise<CollectionEntry<"changelog">[]> {
  const all = await getCollection("changelog", visible);
  return all.sort((a, b) => compareVersions(b.data.version, a.data.version));
}

export const formatDate = (date: Date) =>
  date.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" });

export const isoDate = (date: Date) => date.toISOString().slice(0, 10);
