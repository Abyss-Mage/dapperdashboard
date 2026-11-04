import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import '../legal/legal.css';
import LegalPage from '../legal/LegalPage';

export const metadata = { title: 'Privacy policy', description: 'The Privacy policy of Dapper Detailing Pros, Peterborough.', alternates: { canonical: '/privacy-policy' } };

export default function Page() {
  return <LegalPage page="privacy-policy" />;
}
