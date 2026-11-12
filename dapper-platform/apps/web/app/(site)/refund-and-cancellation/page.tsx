import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import '../legal/legal.css';
import LegalPage from '../legal/LegalPage';

const base: Metadata = { title: 'Refund & cancellation policy', description: 'The Refund & cancellation policy of Dapper Detailing Pros, Peterborough.', alternates: { canonical: '/refund-and-cancellation' } };
export async function generateMetadata() { return pageMetadata('/refund-and-cancellation', base); }

export default function Page() {
  return <LegalPage page="refund-and-cancellation" />;
}
