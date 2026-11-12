import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import '../legal/legal.css';
import LegalPage from '../legal/LegalPage';

const base: Metadata = { title: 'Privacy policy', description: 'The Privacy policy of Dapper Detailing Pros, Peterborough.', alternates: { canonical: '/privacy-policy' } };
export async function generateMetadata() { return pageMetadata('/privacy-policy', base); }

export default function Page() {
  return <LegalPage page="privacy-policy" />;
}
