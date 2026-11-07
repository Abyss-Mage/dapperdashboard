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

const indexing = process.env.DAPPER_PUBLIC_INDEXING === 'enabled';

export const metadata: Metadata = {
  metadataBase: new URL('https://dapperdetailing.ca'),
  title: { default: 'Dapper Detailing Pros | Vehicle care in Peterborough', template: '%s | Dapper Detailing Pros' },
  description: 'Detailing, ceramic coating, paint protection film and window tint in Peterborough, Ontario.',
  icons: { icon: '/site/assets/favicon.webp' },
  robots: indexing ? { index: true, follow: true } : { index: false, follow: false },
  openGraph: { type: 'website', images: ['/site/assets/services/suv.webp'] },
  twitter: { card: 'summary_large_image' },
};

export const viewport: Viewport = { themeColor: '#f6f4f1', width: 'device-width', initialScale: 1 };

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-CA">
      <body>
        <a className="d-skip" href="#main">Skip to content</a>
        <SiteHeader />
        <main id="main" tabIndex={-1}>{children}</main>
        <SiteFooter />
        <SmoothScroll />
        <Tracking />
      </body>
    </html>
  );
}
