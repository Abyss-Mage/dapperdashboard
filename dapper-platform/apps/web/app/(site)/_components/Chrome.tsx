import Link from 'next/link';
import { nav, studio } from '../content';
import Reveal from './Reveal';
import { ArrowUpRight } from './Icons';

// The last panel of every page: one line, then the ways forward written on a strip of tape laid across the page.
export function Close({ before = 'Your vehicle.', after = 'Our next conversation.' }: { before?: string; after?: string }) {
  return (
    <Reveal className="close" aria-labelledby="close-title">
      <h2 id="close-title" data-split>{before}<br /><span className="after">{after}</span></h2>
      <nav className="close__strip" data-lay aria-label="Next steps">
        <a href="/booking">Book an appointment<ArrowUpRight size={22} aria-hidden /></a>
        <a className="num" href={'tel:' + studio.tel}>Call {studio.phone}</a>
        <Link href="/contact-us">Talk to the team</Link>
      </nav>
    </Reveal>
  );
}

export function Footer() {
  return (
    <footer className="site-footer">
      <div>
        <Link href="/" className="brand"><img src="/site/assets/logo.webp" width={30} height={30} alt="" /><span>DAPPER<small>DETAILING PROS</small></span></Link>
        <p style={{ marginTop: 16 }}>© 2026 Dapper Detailing Pros Ltd.</p>
      </div>
      <address>
        <span>{studio.street}</span>
        <span>{studio.city}</span>
        <a href={'tel:' + studio.tel}>{studio.phone}</a>
        <a href={'mailto:' + studio.email}>{studio.email}</a>
      </address>
      <nav aria-label="Footer">
        {nav.map((item) => <Link key={item.href} href={item.href}>{item.label}</Link>)}
        <a href="/account">Customer account</a>
        <a href="/booking">Appointments</a>
      </nav>
      <div className="fine">
        <p>Before views are illustrations made from the same photograph; visual effects show services, not measured outcomes. Confirm available services and suitable options with the team.</p>
        <p>“2004 Porsche Carrera GT” by <a href="https://sketchfab.com/3d-models/2004-porsche-carrera-gt-a43be10ac0174dc0a35984c2d41d4058" target="_blank" rel="noopener">Ddiaz Design</a> · <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener">CC BY 4.0</a>. Manufacturer affiliation is not implied.</p>
      </div>
    </footer>
  );
}
