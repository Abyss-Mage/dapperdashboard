import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import '../legal/legal.css';
import LegalPage from '../legal/LegalPage';

const base: Metadata = { title: 'Terms & conditions', description: 'The Terms & conditions of Dapper Detailing Pros, Peterborough.', alternates: { canonical: '/terms-conditions' } };
export async function generateMetadata() { return pageMetadata('/terms-conditions', base); }

export default function Page() {
  return <LegalPage page="terms-conditions" />;
}
