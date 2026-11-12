import type { Metadata } from 'next';
import { pageMetadata } from '../_components/seo';
import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import ContactPage from './ContactPage';

const base: Metadata = { title: 'Contact Us', description: 'Contact Dapper Detailing Pros at 660 The Kingsway, Peterborough. Ask a question, request a quote or ask about mobile service.', alternates: { canonical: '/contact-us' } };
export async function generateMetadata() { return pageMetadata('/contact-us', base); }

export default function Contact() {
  return <ContactPage />;
}
