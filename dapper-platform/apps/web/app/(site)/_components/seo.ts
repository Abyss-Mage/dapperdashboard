import type { Metadata } from 'next';
import { SITE_SLUG } from './site';
import type { SiteSEO } from '../../seo-pages';

/* The owner's search settings (owner 2026-11-13), read on the server from the API on this machine and refreshed every minute.
   If they cannot be read, every page keeps its own title and description. */
export async function siteSEO(): Promise<SiteSEO | null> {
  try {
    const r = await fetch(`http://127.0.0.1:8121/api/v1/public/sites/${SITE_SLUG}`, { next: { revalidate: 60 }, headers: { Accept: 'application/json' } });
    if (!r.ok) return null;
    return ((await r.json()) as { seo?: SiteSEO }).seo ?? null;
  } catch { return null; }
}

const words = (v?: string) => (v ?? '').split(',').map((s) => s.trim()).filter(Boolean);

/* One page's metadata: the owner's text for that page, else the page's own; keywords and share image fall back to the site-wide ones. */
export async function pageMetadata(path: string, base: Metadata): Promise<Metadata> {
  const seo = await siteSEO();
  const p = seo?.pages?.[path];
  const out: Metadata = { ...base };
  if (p?.title) out.title = { absolute: p.title };
  if (p?.description) out.description = p.description;
  const keywords = words(p?.keywords || seo?.keywords);
  if (keywords.length) out.keywords = keywords;
  const image = p?.image || seo?.image;
  if (image) out.openGraph = { type: 'website', images: [image] };
  if (p?.hidden) out.robots = { index: false, follow: true };
  return out;
}
