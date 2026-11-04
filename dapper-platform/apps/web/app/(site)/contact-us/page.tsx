import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import ContactPage from './ContactPage';

export const metadata = { title: 'Contact Us', description: 'Contact Dapper Detailing Pros at 660 The Kingsway, Peterborough. Ask a question, request a quote or ask about mobile service.', alternates: { canonical: '/contact-us' } };

export default function Contact() {
  return <ContactPage />;
}
