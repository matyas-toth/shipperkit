#!/usr/bin/env node
/*
  Start a changelog post from the release notes release-please writes to
  CHANGELOG.md.

    npm run changelog                  draft a post for the newest release
    npm run changelog -- 1.0.0-rc.42   draft a post for one release
    npm run changelog -- --missing     list releases without a post

  The draft lands in src/content/changelog/<version>.mdx with draft: true and
  the raw notes in a comment. Rewrite them as prose for people who run Shipper,
  fill in title and description, and remove draft: true to publish.
*/
import { existsSync } from "node:fs";
import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../../", import.meta.url);
const posts = new URL("../src/content/changelog/", import.meta.url);
const arg = process.argv[2];

const source = await readFile(new URL("CHANGELOG.md", root), "utf8");
const heading = /^##\s+\[?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)\]?(?:\([^)]*\))?\s*\((\d{4}-\d{2}-\d{2})\)\s*$/gm;
const releases = [];
for (let match; (match = heading.exec(source)); ) {
  if (releases.length) releases.at(-1).end = match.index;
  releases.push({ version: match[1], date: match[2], start: heading.lastIndex, end: source.length });
}
if (!releases.length) {
  console.error("No releases found in CHANGELOG.md");
  process.exit(1);
}

const postPath = (version) => new URL(`${version}.mdx`, posts);

if (arg === "--missing") {
  const missing = releases.filter((r) => !existsSync(postPath(r.version)));
  console.log(missing.length ? missing.map((r) => `${r.version}  ${r.date}`).join("\n") : "Every release has a post.");
  process.exit(0);
}

const release = arg ? releases.find((r) => r.version === arg.replace(/^v/, "")) : releases[0];
if (!release) {
  console.error(`${arg} is not in CHANGELOG.md`);
  process.exit(1);
}
const target = postPath(release.version);
if (existsSync(target)) {
  console.error(`${fileURLToPath(target)} already exists`);
  process.exit(1);
}

// Keep the section headings and bullets, drop commit hashes, keep PR numbers.
const notes = source
  .slice(release.start, release.end)
  .split("\n")
  .map((line) => line.replace(/\s*\(\[[0-9a-f]{7,}\]\([^)]*\)\)/g, "").replace(/\(\[#(\d+)\]\([^)]*\)\)/g, "(#$1)").trimEnd())
  .filter(Boolean)
  .join("\n")
  .replaceAll("*/", "* /");

await writeFile(
  target,
  `---
version: ${release.version}
date: ${release.date}
title: ""
description: ""
draft: true
---

{/*
Release notes from CHANGELOG.md. PR descriptions on GitHub have the detail.

${notes}
*/}
`,
);
console.log(fileURLToPath(target));
