import type { MetadataRoute } from 'next';
import { headers } from 'next/headers';
import { APP_SITES, MAIN_SITE } from './seo-pages';

export default async function robots(): Promise<MetadataRoute.Robots> {
  const host = ((await headers()).get('host') ?? '').toLowerCase();
  const app = APP_SITES.find((a) => a.host === host);
  // The dashboard and workspace subdomains: only the sign-in page may be listed; everything behind it stays out of search.
  if (app) return { rules: { userAgent: '*', allow: app.paths.map((p) => `${p}$`), disallow: '/' }, sitemap: `https://${app.host}/sitemap.xml` };
  if (process.env.DAPPER_PUBLIC_INDEXING !== 'enabled') return { rules: { userAgent: '*', disallow: '/' } };
  return { rules: { userAgent: '*', allow: '/', disallow: ['/workspace', '/dashboard', '/owner', '/content', '/account', '/book-appointment', '/enroll', '/pay/', '/shop', '/dealer', '/api/'] }, sitemap: `${MAIN_SITE}/sitemap.xml` };
}
