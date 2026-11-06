import '@fontsource-variable/unbounded/index.css';
import { SiteHeader } from '../../_chrome/SiteHeader';
import { SiteFooter } from '../../_chrome/SiteFooter';
import PublicBookingWizard from '../workspace/PublicBookingWizard';
import ForceDark from '../../_chrome/ForceDark';
import styles from './booking.module.css';

export const metadata = { title: 'Book now | Dapper Detailing Pros', description: 'Choose mobile service or at-shop detailing and pick your date and time with Dapper Detailing Pros in Peterborough.' };

/* The booking steps. Opened on its own it wears the site's header, a film-frame headline and footer; with ?embed=1 (the popup that every
   Book now button opens) it is only the steps, in the dark theme, so the popup is just the booking. */
export default async function PublicBookingPortal({ searchParams }: { searchParams: Promise<{ embed?: string }> }) {
  const embed = (await searchParams).embed === '1';
  if (embed) {
    return (
      <div className={styles.embed}>
        <ForceDark />
        <PublicBookingWizard />
      </div>
    );
  }
  return (
    <div className={styles.page}>
      <SiteHeader />
      <section className={styles.hero} aria-labelledby="book-title">
        <p className={styles.kicker}>Dapper Detailing Pros · Peterborough</p>
        <h1 id="book-title">Book your <em>detail.</em></h1>
        <p className={styles.lede}>Choose mobile service or at-shop detailing, then pick your date and time in a few quick steps. Already have an account? <a href="/account">Sign in</a>.</p>
      </section>
      <main className={styles.sheet}>
        <PublicBookingWizard />
      </main>
      <SiteFooter />
    </div>
  );
}
