import type { Metadata } from 'next';
import { pageMetadata } from './_components/seo';
import './paper.css';
import './services/services.css';
import './[service]/vehicle.css';
import './home/home.css';
import HomePage from './home/HomePage';

const base: Metadata = { title: { absolute: 'Dapper Detailing Pros | Vehicle care in Peterborough' }, alternates: { canonical: '/' } };
export async function generateMetadata() { return pageMetadata('/', base); }

export default function Home() {
  return <HomePage />;
}
