import { categories } from './(site)/content';

/* The public website's pages, for the sitemap and the owner's search settings. The dashboard and workspace subdomains list only their
   sign-in page (owner 2026-11-13: everything behind sign-in stays out of search). */
export const MAIN_SITE = 'https://dapperdetailing.ca';
export const APP_SITES = [
  { host: 'dashboard.dapperdetailing.ca', label: 'Owner dashboard', paths: ['/dashboard'] },
  { host: 'workspace.dapperdetailing.ca', label: 'Staff workspace', paths: ['/workspace'] },
];

export const sitePages: { path: string; label: string }[] = [
  { path: '/', label: 'Home' }, { path: '/services', label: 'Services' }, { path: '/portfolio', label: 'Portfolio' },
  { path: '/about-us', label: 'About us' }, { path: '/contact-us', label: 'Contact us' },
  ...categories.map((c) => ({ path: '/' + c.slug, label: c.title })),
  { path: '/privacy-policy', label: 'Privacy policy' }, { path: '/refund-and-cancellation', label: 'Refund & cancellation' }, { path: '/terms-conditions', label: 'Terms & conditions' },
];

export type PageSEO = { title: string; description: string; keywords: string; image: string; hidden: boolean };
export type SiteSEO = { title: string; description: string; keywords: string; image: string; googleVerification: string; bingVerification: string; pages: Record<string, PageSEO> | null };
