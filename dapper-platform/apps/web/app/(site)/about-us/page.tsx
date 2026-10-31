import '@fontsource-variable/archivo/index.css';
import '../paper.css';
import './about.css';
import AboutPage from './AboutPage';

export const metadata = { title: 'About Us', description: 'The people and story behind Dapper Detailing Pros in Peterborough.', alternates: { canonical: '/about-us' } };

export default function About() {
  return <AboutPage />;
}
