import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import '../legal/legal.css';
import LegalPage from '../legal/LegalPage';

export const metadata = { title: 'Terms & conditions', description: 'The Terms & conditions of Dapper Detailing Pros, Peterborough.', alternates: { canonical: '/terms-conditions' } };

export default function Page() {
  return <LegalPage page="terms-conditions" />;
}
