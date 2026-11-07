import '@fontsource-variable/unbounded/index.css';
import '../paper.css';
import '../legal/legal.css';
import LegalPage from '../legal/LegalPage';

export const metadata = { title: 'Refund & cancellation policy', description: 'The Refund & cancellation policy of Dapper Detailing Pros, Peterborough.', alternates: { canonical: '/refund-and-cancellation' } };

export default function Page() {
  return <LegalPage page="refund-and-cancellation" />;
}
