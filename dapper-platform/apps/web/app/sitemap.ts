import type { MetadataRoute } from 'next';
import { headers } from 'next/headers';
import { APP_SITES, MAIN_SITE, sitePages } from './seo-pages';
import { siteSEO } from './(site)/_components/seo';

/* One XML sitemap per address (owner 2026-11-13): the dashboard and workspace subdomains list only their sign-in page; the website lists
   its public pages, without the ones hidden from search in the dashboard. */
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const host = ((await headers()).get('host') ?? '').toLowerCase();
  const app = APP_SITES.find((a) => a.host === host);
  if (app) return app.paths.map((p) => ({ url: `https://${app.host}${p}` }));
  if (process.env.DAPPER_PUBLIC_INDEXING !== 'enabled') return [];
  const hidden = (await siteSEO())?.pages ?? {};
  return [...sitePages.map((p) => p.path), '/studio'].filter((p) => !hidden[p]?.hidden).map((p) => ({ url: MAIN_SITE + (p === '/' ? '' : p) }));
}
