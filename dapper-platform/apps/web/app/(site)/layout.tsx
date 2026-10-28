import type { Metadata, Viewport } from 'next';
import '@fontsource/manrope/400.css';
import '@fontsource/manrope/500.css';
import '@fontsource/manrope/600.css';
import '@fontsource/manrope/700.css';
import '@fontsource/barlow-condensed/600.css';
import '@fontsource/barlow-condensed/700.css';
import '@fontsource/barlow-condensed/800.css';
import './site.css';
import './story.css';
import Header from './_components/Header';
import SmoothScroll from './_components/SmoothScroll';
import { Footer } from './_components/Chrome';

const indexing = process.env.DAPPER_PUBLIC_INDEXING === 'enabled';

export const metadata: Metadata = {
  metadataBase: new URL('https://dapperdetailing.ca'),
  title: { default: 'Dapper Detailing Pros | Vehicle care in Peterborough', template: '%s | Dapper Detailing Pros' },
  description: 'Detailing, ceramic coating, paint protection film and window tint at 660 The Kingsway, Peterborough, Ontario.',
  icons: { icon: '/site/assets/favicon.webp' },
  robots: indexing ? { index: true, follow: true } : { index: false, follow: false },
  openGraph: { type: 'website', images: ['/site/assets/hero.webp'] },
  twitter: { card: 'summary_large_image' },
};

export const viewport: Viewport = { themeColor: '#09090b', width: 'device-width', initialScale: 1 };

// The home story's 3D fallback is an ES module that imports "three" by name; this map tells the browser where it lives.
const importMap = JSON.stringify({ imports: { three: '/site/assets/three/three.module.js', 'three/addons/': '/site/assets/three/addons/' } });

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-CA">
      <head>
        <script type="importmap" dangerouslySetInnerHTML={{ __html: importMap }} />
      </head>
      <body>
        <a className="skip" href="#main">Skip to content</a>
        <Header />
        <main id="main" tabIndex={-1}>{children}</main>
        <Footer />
        <SmoothScroll />
      </body>
    </html>
  );
}
