import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import '@fontsource-variable/unbounded/index.css';
import { contact } from '../(site)/dapper';
import s from './chrome.module.css';

/* The one footer: logo and a line about the work, how to reach us, the small print and the page links, and the name across the bottom.
   No city or street address (more locations are coming): visitors are pointed to directions instead. */
export function SiteFooter() {
  return (
    <div className={s.footWrap}><footer className={s.footer}>
      <div className={s.fMain}>
        <div className={s.fLeft}>
          <Link href="/" aria-label="Dapper Detailing Pros, home"><img src="/site/assets/logo.webp" alt="" width={104} height={104} loading="lazy" /></Link>
          <p>Careful detailing, coatings and protection for cars, trucks, boats and aircraft.</p>
          <h2>Get in touch</h2>
          <a className={s.fLink} href={'mailto:' + contact.email}>{contact.email}<ArrowUpRight size={14} aria-hidden="true" /></a>
          <a className={s.fLink} href={'tel:' + contact.tel}>{contact.phone}</a>
          <span className={s.fSmall}>
            <a href={contact.directions} target="_blank" rel="noopener noreferrer">Get directions</a>
            <a href={contact.instagram} target="_blank" rel="noopener noreferrer">Instagram</a>
            <a href={contact.youtube} target="_blank" rel="noopener noreferrer">YouTube</a>
          </span>
        </div>
        <div className={s.fRight}>
          <p className={s.fCopy}>
            <span>© 2026 Dapper Detailing Pros Ltd. All rights reserved.</span>
            <a href="/privacy-policy">Privacy policy</a>
            <a href="/terms-conditions">Terms &amp; conditions</a>
            <a href="/refund-and-cancellation">Refund &amp; cancellation</a>
          </p>
          <nav className={s.fNav} aria-label="Footer">
            <Link href="/">Home<ArrowUpRight size={13} aria-hidden="true" /></Link>
            <Link href="/about-us">About us<ArrowUpRight size={13} aria-hidden="true" /></Link>
            <Link href="/services">Our services<ArrowUpRight size={13} aria-hidden="true" /></Link>
            <Link href="/portfolio">Portfolio<ArrowUpRight size={13} aria-hidden="true" /></Link>
            <Link href="/contact-us">Contact us<ArrowUpRight size={13} aria-hidden="true" /></Link>
            <Link href="/account">My account<ArrowUpRight size={13} aria-hidden="true" /></Link>
          </nav>
        </div>
      </div>
      <p className={s.mark} aria-hidden="true"><span>Dapper</span> <span>Detailing</span></p>
    </footer></div>
  );
}
