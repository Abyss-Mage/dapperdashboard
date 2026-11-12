import '@fontsource-variable/unbounded/index.css';
import { SiteHeader } from '../../../_chrome/SiteHeader';
import { SiteFooter } from '../../../_chrome/SiteFooter';
import Tracking from '../../../(site)/_components/Tracking';
import ThankYou from './ThankYou';
import styles from '../booking.module.css';

export const metadata = { title: 'Booking confirmed | Dapper Detailing Pros', robots: { index: false, follow: false } };

/* Where a finished booking lands. The Google tag and the Meta Pixel report the purchase from here (see ThankYou). */
export default function ThankYouPage() {
  return (
    <div className={styles.page}>
      <Tracking />
      <SiteHeader />
      <ThankYou />
      <SiteFooter />
    </div>
  );
}
