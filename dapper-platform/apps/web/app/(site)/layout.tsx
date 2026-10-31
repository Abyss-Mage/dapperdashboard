import type { Metadata, Viewport } from 'next';
import '@fontsource/sora/700.css';
import '@fontsource/sora/800.css';
import '@fontsource/manrope/400.css';
import '@fontsource/manrope/500.css';
import '@fontsource/manrope/600.css';
import '@fontsource/manrope/700.css';
import './_v3/v3.css';
import { Nav3, Footer3 } from './_v3/Chrome3';
import SmoothScroll from './_components/SmoothScroll';
import Tracking from './_components/Tracking';

const indexing = process.env.DAPPER_PUBLIC_INDEXING === 'enabled';

export const metadata: Metadata = {
  metadataBase: new URL('https://dapperdetailing.ca'),
  title: { default: 'Dapper Detailing Pros | Vehicle care in Peterborough', template: '%s | Dapper Detailing Pros' },
  description: 'Detailing, ceramic coating, paint protection film and window tint at 660 The Kingsway, Peterborough, Ontario.',
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
        <Nav3 />
        <main id="main" tabIndex={-1}>{children}</main>
        <Footer3 />
        <SmoothScroll />
        <Tracking />
      </body>
    </html>
  );
}
