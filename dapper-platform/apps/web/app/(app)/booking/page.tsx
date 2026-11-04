import '@fontsource-variable/unbounded/index.css';
import ThemeToggle from '../ThemeToggle';
import PublicBookingWizard from '../workspace/PublicBookingWizard';
import styles from './booking.module.css';

export const metadata = { title: 'Book now | Dapper Detailing Pros', description: 'Choose mobile service or at-shop detailing and pick your date and time with Dapper Detailing Pros in Peterborough.' };

const links = [['/', 'Home'], ['/services', 'Services'], ['/portfolio', 'Portfolio'], ['/about-us', 'About'], ['/contact-us', 'Contact']];

/* The booking page wears the website's look: the black strip header, a headline band, then the booking steps on a calm sheet, then the site footer line. */
export default function PublicBookingPortal() {
  return (
    <div className={styles.page}>
      <header className={styles.bar}>
        <a href="/" className={styles.brand} aria-label="Dapper Detailing Pros, home">Dapper</a>
        <nav aria-label="Main">{links.map(([href, label]) => <a key={href} href={href}>{label}</a>)}</nav>
        <ThemeToggle className={styles.theme} compact />
      </header>
      <section className={styles.hero} aria-labelledby="book-title">
        <p className={styles.kicker}>Dapper Detailing Pros · Peterborough</p>
        <h1 id="book-title">Book your <em>detail.</em></h1>
        <p className={styles.lede}>Choose mobile service or at-shop detailing, then pick your date and time in a few quick steps. Already have an account? <a href="/account">Sign in</a>.</p>
      </section>
      <main className={styles.sheet}>
        <PublicBookingWizard />
      </main>
      <footer className={styles.foot}>
        <span>© 2026 Dapper Detailing Pros Ltd.</span>
        <span><a href="/privacy-policy">Privacy policy</a><a href="/terms-conditions">Terms &amp; conditions</a><a href="/refund-and-cancellation">Refund &amp; cancellation</a></span>
      </footer>
    </div>
  );
}
