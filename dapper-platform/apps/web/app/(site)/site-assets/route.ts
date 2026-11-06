import { readdir, stat } from 'node:fs/promises';
import path from 'node:path';

/* The list of the website's own pictures and films (everything under public/site/assets except the hero frames), so the dashboard can
   show each one and let the owner replace it. Only file names and sizes: nothing private is in that folder. */
export const dynamic = 'force-dynamic';

const ROOT = path.join(process.cwd(), 'public', 'site', 'assets');
const KIND: Record<string, 'image' | 'video'> = { '.jpg': 'image', '.jpeg': 'image', '.png': 'image', '.webp': 'image', '.mp4': 'video', '.webm': 'video' };
const SKIP = new Set(['frames', 'three', 'env', 'map']);

type Item = { path: string; kind: 'image' | 'video'; bytes: number };

async function walk(dir: string, rel: string, out: Item[]) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const next = rel ? rel + '/' + entry.name : entry.name;
    if (entry.isDirectory()) { if (!SKIP.has(entry.name)) await walk(path.join(dir, entry.name), next, out); continue; }
    const kind = KIND[path.extname(entry.name).toLowerCase()];
    if (kind) out.push({ path: next, kind, bytes: (await stat(path.join(dir, entry.name))).size });
  }
}

export async function GET() {
  const items: Item[] = [];
  await walk(ROOT, '', items).catch(() => undefined);
  items.sort((a, b) => a.path.localeCompare(b.path));
  return Response.json({ items }, { headers: { 'Cache-Control': 'no-store' } });
}
