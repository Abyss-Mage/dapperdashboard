import '@fontsource-variable/unbounded/index.css';
import { SiteHeader } from '../../_chrome/SiteHeader';
import { SiteFooter } from '../../_chrome/SiteFooter';
import PublicBookingWizard from '../workspace/PublicBookingWizard';
import Tracking from '../../(site)/_components/Tracking';
import styles from './booking.module.css';

export const metadata = { title: 'Book now | Dapper Detailing Pros', description: 'Choose mobile service or at-shop detailing and pick your date and time with Dapper Detailing Pros in Peterborough.' };

/* The booking page: the site's header, a film-frame headline, the booking steps and the footer. It is a page of its own (not a popup) so
   the ad tags see the visit, and a finished booking sends the customer on to /book-appointment/thank-you. */
export default function PublicBookingPortal() {
  return (
    <div className={styles.page}>
      <Tracking />
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
