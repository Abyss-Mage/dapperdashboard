import '@fontsource-variable/archivo/index.css';
import '../paper.css';
import './portfolio.css';
import PortfolioPage from './PortfolioPage';

export const metadata = { title: 'Portfolio', description: 'Real vehicles cared for by Dapper Detailing Pros in Peterborough.', alternates: { canonical: '/portfolio' } };

export default function Portfolio() {
  return <PortfolioPage />;
}
