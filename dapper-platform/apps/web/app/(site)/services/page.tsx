import '@fontsource/barlow-condensed/600.css';
import '@fontsource/barlow-condensed/700.css';
import '@fontsource/barlow-condensed/800.css';
import '../paper.css';
import './services.css';
import ServicesPage from './ServicesPage';

export const metadata = { title: 'Services', description: 'Detailing, ceramic coating and paint protection film for every vehicle, in Peterborough.', alternates: { canonical: '/services' } };

export default function Services() {
  return <ServicesPage />;
}
