import Link from 'next/link';
import '@fontsource-variable/unbounded/index.css';
import { categories } from '../(site)/content';
import { contact } from '../(site)/dapper';
import s from './chrome.module.css';

/* The one footer: the booking call, then the logo with a line about the work, the services, the company links and how to reach us, and the
   legal line. No city or street address (more locations are coming): visitors are pointed to the contact page and directions instead. */
export function SiteFooter() {
  return (
    <footer className={s.footer}>
      <div className={s.fGrid}>
        <div className={s.fBrand}>
          <Link href="/" aria-label="Dapper Detailing Pros, home"><img src="/site/assets/logo.webp" alt="" width={96} height={96} loading="lazy" /></Link>
          <p>Careful detailing, coatings and protection for cars, trucks, boats and aircraft.</p>
        </div>
        <nav aria-label="Services" className={s.fCol}>
          <h2>Services</h2>
          <div className={s.fTwo}>
            {categories.map((c) => <Link key={c.key} href={'/' + c.slug}>{c.short}</Link>)}
          </div>
        </nav>
        <nav aria-label="Company" className={s.fCol}>
          <h2>Company</h2>
          <Link href="/portfolio">Portfolio</Link>
          <Link href="/about-us">About us</Link>
          <Link href="/contact-us">Contact us</Link>
          <Link href="/account">My account</Link>
        </nav>
        <div className={s.fCol}>
          <h2>Reach us</h2>
          <a href={'tel:' + contact.tel}>{contact.phone}</a>
          <a href={'mailto:' + contact.email}>{contact.email}</a>
          <a href={contact.directions} target="_blank" rel="noopener noreferrer">Get directions</a>
          <a href={contact.instagram} target="_blank" rel="noopener noreferrer">Instagram</a>
          <a href={contact.youtube} target="_blank" rel="noopener noreferrer">YouTube</a>
        </div>
      </div>
      <p className={s.mark} aria-hidden="true">Dapper</p>
      <div className={s.legal}>
        <span>© 2026 Dapper Detailing Pros Ltd. All rights reserved.</span>
        <span><a href="/privacy-policy">Privacy policy</a><a href="/terms-conditions">Terms &amp; conditions</a><a href="/refund-and-cancellation">Refund &amp; cancellation</a></span>
      </div>
    </footer>
  );
}
