import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '../paper.css';
import './services.css';
import ServicesPage from './ServicesPage';

const base: Metadata = { title: 'Services', description: 'Detailing, ceramic coating and paint protection film for every vehicle, in Peterborough.', alternates: { canonical: '/services' } };
export async function generateMetadata() { return pageMetadata('/services', base); }

export default function Services() {
  return <ServicesPage />;
}
