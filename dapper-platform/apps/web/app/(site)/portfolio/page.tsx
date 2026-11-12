import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import './portfolio.css';
import PortfolioPage from './PortfolioPage';

const base: Metadata = { title: 'Portfolio', description: 'Real vehicles cared for by Dapper Detailing Pros in Peterborough.', alternates: { canonical: '/portfolio' } };
export async function generateMetadata() { return pageMetadata('/portfolio', base); }

export default function Portfolio() {
  return <PortfolioPage />;
}
