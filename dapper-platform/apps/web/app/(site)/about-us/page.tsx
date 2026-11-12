import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import './about.css';
import AboutPage from './AboutPage';

const base: Metadata = { title: 'About Us', description: 'The people and story behind Dapper Detailing Pros in Peterborough.', alternates: { canonical: '/about-us' } };
export async function generateMetadata() { return pageMetadata('/about-us', base); }

export default function About() {
  return <AboutPage />;
}
