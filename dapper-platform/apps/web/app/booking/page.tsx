import ThemeToggle from '../ThemeToggle';
import PublicBookingWizard from '../workspace/PublicBookingWizard';

export const metadata = { title: 'Book an appointment | Dapper Detailing Pros' };

export default function PublicBookingPortal() {
  return (
    <main className="secure-workspace" style={{ maxWidth: 820, margin: '0 auto', padding: '1.5rem' }}>
      <header style={{ display: 'flex', flexWrap: 'wrap', gap: '1rem', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <a href="/" className="brand" style={{ textDecoration: 'none' }}>
          <span className="brandmark">D<span>↗</span></span>
          <span>DAPPER<small>DETAILING PROS</small></span>
        </a>
        <ThemeToggle />
      </header>
      <h1 style={{ margin: '0 0 .25rem' }}>Book an appointment</h1>
      <p style={{ margin: '0 0 1.25rem', color: 'var(--text-muted)' }}>A few quick questions and your time is held. Already have an account? <a href="/account">Sign in</a>.</p>
      <PublicBookingWizard />
    </main>
  );
}
