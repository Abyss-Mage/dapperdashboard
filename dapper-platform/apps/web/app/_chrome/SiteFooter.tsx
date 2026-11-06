import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import '@fontsource-variable/unbounded/index.css';
import { contact } from '../(site)/dapper';
import s from './chrome.module.css';

/* The one footer: the booking call, four columns, the name at full width and the legal line. No street address (more locations are coming):
   the visitor is pointed to the contact page and directions instead. */
export function SiteFooter() {
  return (
    <footer className={s.footer}>
      <div className={s.cta}>
        <p className={s.lead}>Ready for <em>precise detailing?</em></p>
        <div className={s.ctaRow}>
          <a className={s.book} href="/booking">Book now<ArrowRight size={18} aria-hidden="true" /></a>
          <a className={s.call} href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
        </div>
      </div>
      <div className={s.cols}>
        <div><h2>Visit</h2><p>Peterborough, Ontario</p><a href={contact.directions} target="_blank" rel="noopener noreferrer">Get directions</a></div>
        <div><h2>Contact</h2><a href={'tel:' + contact.tel}>{contact.phone}</a><a href={'mailto:' + contact.email}>{contact.email}</a></div>
        <div><h2>Follow</h2><a href={contact.instagram} target="_blank" rel="noopener noreferrer">Instagram</a><a href={contact.youtube} target="_blank" rel="noopener noreferrer">YouTube</a></div>
        <div><h2>Explore</h2><Link href="/services">Services</Link><Link href="/portfolio">Portfolio</Link><Link href="/about-us">About us</Link><Link href="/contact-us">Contact us</Link><Link href="/account">My account</Link></div>
      </div>
      <p className={s.mark} aria-hidden="true">Dapper</p>
      <div className={s.legal}>
        <span>© 2026 Dapper Detailing Pros Ltd. All rights reserved.</span>
        <span><a href="/privacy-policy">Privacy policy</a><a href="/terms-conditions">Terms &amp; conditions</a><a href="/refund-and-cancellation">Refund &amp; cancellation</a></span>
      </div>
    </footer>
  );
}
