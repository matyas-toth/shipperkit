import { defineCollection } from "astro:content";
import { glob } from "astro/loaders";
import { z } from "astro/zod";

/* Site content written as MDX in src/content. The file name is the URL:
   blog/<slug>.mdx, compare/<slug>.mdx and changelog/<version>.mdx.
   Entries with draft: true render in `astro dev` and are left out of builds. */

const keepFileName = ({ entry }: { entry: string }) => entry.replace(/\.mdx?$/, "");

const blog = defineCollection({
  loader: glob({ base: "./src/content/blog", pattern: "*.mdx", generateId: keepFileName }),
  schema: z.object({
    title: z.string(),
    description: z.string().max(200),
    date: z.coerce.date(),
    author: z.string().default("Mátyás Tóth"),
    draft: z.boolean().default(false),
  }),
});

const compare = defineCollection({
  loader: glob({ base: "./src/content/compare", pattern: "*.mdx", generateId: keepFileName }),
  schema: z.object({
    /** The other product, as its makers write it. */
    name: z.string(),
    description: z.string().max(200),
    /** When the facts about the other product were last checked against its own pages. */
    checked: z.coerce.date(),
    order: z.number().int(),
    rows: z.array(z.object({ label: z.string(), shipper: z.string(), them: z.string() })).min(4),
    sources: z.array(z.url()).min(1),
    draft: z.boolean().default(false),
  }),
});

const changelog = defineCollection({
  loader: glob({ base: "./src/content/changelog", pattern: "*.mdx", generateId: keepFileName }),
  schema: z.object({
    version: z.string().regex(/^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$/),
    date: z.coerce.date(),
    title: z.string(),
    description: z.string().max(200),
    draft: z.boolean().default(false),
  }),
});

export const collections = { blog, compare, changelog };
