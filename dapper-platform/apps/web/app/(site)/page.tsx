import './paper.css';
import './services/services.css';
import './[service]/vehicle.css';
import './home/home.css';
import HomePage from './home/HomePage';

export const metadata = { title: { absolute: 'Dapper Detailing Pros | Vehicle care in Peterborough' }, alternates: { canonical: '/' } };

export default function Home() {
  return <HomePage />;
}
