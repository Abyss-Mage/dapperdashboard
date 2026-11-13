import type { Metadata, Viewport } from 'next';
import '@fontsource/sora/700.css';
import '@fontsource-variable/unbounded/index.css';
import '@fontsource/manrope/400.css';
import '@fontsource/manrope/500.css';
import '@fontsource/manrope/600.css';
import '@fontsource/manrope/700.css';
import './_v3/v3.css';
import './footer.css';
import './cut.css';
import { SiteHeader } from '../_chrome/SiteHeader';
import { SiteFooter } from '../_chrome/SiteFooter';
import SmoothScroll from './_components/SmoothScroll';
import Tracking from './_components/Tracking';
import RemovedImages from './_components/RemovedImages';
import SalesStrip from './_components/SalesStrip';
import { siteSEO } from './_components/seo';

const indexing = process.env.DAPPER_PUBLIC_INDEXING === 'enabled';

const base: Metadata = {
  metadataBase: new URL('https://dapperdetailing.ca'),
  title: { default: 'Dapper Detailing Pros | Vehicle care in Peterborough', template: '%s | Dapper Detailing Pros' },
  description: 'Detailing, ceramic coating, paint protection film and window tint in Peterborough, Ontario.',
  icons: { icon: [{ url: '/favicon.ico', sizes: '48x48' }, { url: '/icon-32.png', type: 'image/png', sizes: '32x32' }, { url: '/icon-192.png', type: 'image/png', sizes: '192x192' }], shortcut: '/favicon.ico', apple: { url: '/apple-touch-icon.png', sizes: '180x180' } },
  robots: indexing ? { index: true, follow: true } : { index: false, follow: false },
  openGraph: { type: 'website', images: ['/site/assets/services/suv.webp'] },
  twitter: { card: 'summary_large_image' },
};

// Site-wide search settings from the dashboard (owner 2026-11-13) over the defaults above.
export async function generateMetadata(): Promise<Metadata> {
  const seo = await siteSEO();
  if (!seo) return base;
  const keywords = seo.keywords.split(',').map((k) => k.trim()).filter(Boolean);
  return {
    ...base,
    ...(seo.title ? { title: { default: seo.title, template: '%s | Dapper Detailing Pros' } } : {}),
    ...(seo.description ? { description: seo.description } : {}),
    ...(keywords.length ? { keywords } : {}),
    ...(seo.image ? { openGraph: { type: 'website', images: [seo.image] } } : {}),
    ...(seo.googleVerification || seo.bingVerification ? { verification: { ...(seo.googleVerification ? { google: seo.googleVerification } : {}), ...(seo.bingVerification ? { other: { 'msvalidate.01': seo.bingVerification } } : {}) } } : {}),
  };
}

export const viewport: Viewport = { themeColor: '#f6f4f1', width: 'device-width', initialScale: 1 };

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-CA">
      <body>
        <a className="d-skip" href="#main">Skip to content</a>
        <SalesStrip />
        <SiteHeader />
        <main id="main" tabIndex={-1}>{children}</main>
        <SiteFooter />
        <SmoothScroll />
        <Tracking />
        <RemovedImages />
      </body>
    </html>
  );
}
