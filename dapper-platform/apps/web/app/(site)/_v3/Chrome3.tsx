'use client';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { ArrowRight, CalendarCheck, Menu, Phone, Wrench, X } from 'lucide-react';
import { categories } from '../content';
import { contact } from '../dapper';

const links = [
  { href: '/services', label: 'Services' },
  { href: '/#work', label: 'Transformations' },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/about-us', label: 'About us' },
  { href: '/contact-us', label: 'Contact us' },
];

// Contact's paper header: the five pages in the comp's order.
const paperLinks = [
  { href: '/', label: 'Home' },
  { href: '/services', label: 'Services' },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/about-us', label: 'About' },
  { href: '/contact-us', label: 'Contact' },
];

// Floating pill navigation (solid once the page scrolls), a full-screen menu and an app-like dock on phones.
export function Nav3() {
  const [solid, setSolid] = useState(false);
  const [open, setOpen] = useState(false);
  const pathname = usePathname();
  // The vehicle pages (/sedan-detailing and the rest) belong to Services; every redesigned page wears the paper header.
  const onVehicle = categories.some((c) => '/' + c.slug === pathname);
  const paper = onVehicle || ['/', '/services', '/contact-us', '/about-us', '/portfolio', '/privacy-policy', '/terms-conditions', '/refund-and-cancellation'].includes(pathname);
  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    const on = () => setSolid(scrollY > 30);
    on(); addEventListener('scroll', on, { passive: true });
    return () => removeEventListener('scroll', on);
  }, []);
  return (
    <>
      <header className={'d-nav' + (solid || open ? ' is-solid' : '') + (paper ? ' d-nav--paper' : '') + (pathname === '/' || pathname === '/services' || onVehicle ? ' d-nav--dark' : '')}>
        {paper
          ? <Link href="/" className="d-brand" aria-label="Dapper Detailing Pros, home">Dapper</Link>
          : <Link href="/" className="d-brand" aria-label="Dapper Detailing Pros, home"><img src="/site/assets/logo.webp" alt="" width={36} height={36} /><span>Dapper<small>Detailing Pros</small></span></Link>}
        <nav aria-label="Main">{(paper ? paperLinks : links).map((l) => <Link key={l.href} href={l.href} aria-current={pathname === l.href ? 'page' : paper && onVehicle && l.href === '/services' ? 'true' : undefined}>{l.label}</Link>)}</nav>
        <a className="d-nav__call" href={'tel:' + contact.tel}><Phone size={15} />{contact.phone}</a>
        {paper
          ? <a className="d-btn d-btn--red" href="/booking"><span className="c-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={20} aria-hidden="true" /></a>
          : <a className="d-btn d-btn--red d-btn--sm" href="/booking">Book now</a>}
        <button type="button" className="d-nav__menu" aria-expanded={open} aria-controls="d-menu" onClick={() => setOpen((o) => !o)}>{open ? <X size={22} /> : <Menu size={22} />}<span className="sr">Menu</span></button>
      </header>
      <div id="d-menu" className={'d-menu' + (open ? ' is-open' : '')} inert={!open}>
        {links.map((l) => <Link key={l.href} href={l.href}>{l.label}</Link>)}
        <a className="d-btn d-btn--red" href="/booking">Book an appointment</a>
      </div>
      <nav className="d-dock" aria-label="Quick actions">
        <Link href="/services"><Wrench size={18} />Services</Link>
        <a className="d-dock__book" href="/booking"><CalendarCheck size={18} />Book now</a>
        <a href={'tel:' + contact.tel}><Phone size={18} />Call</a>
      </nav>
    </>
  );
}

export function Footer3() {
  return (
    <footer className="f-foot">
      <div className="f-foot__cta">
        <p className="f-foot__lead">Ready for <em>precise detailing?</em></p>
        <div className="f-foot__act">
          <a className="f-btn" href="/booking"><span className="f-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
          <a className="f-foot__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
        </div>
      </div>
      <div className="f-foot__cols">
        <div><h2>Visit</h2><p>{contact.street}<br />{contact.city}<br />{contact.country}</p><a href={contact.directions} target="_blank" rel="noopener noreferrer">Get directions</a></div>
        <div><h2>Contact</h2><a href={'tel:' + contact.tel}>{contact.phone}</a><a href={'mailto:' + contact.email}>{contact.email}</a>
          <span className="f-foot__social"><a href={contact.instagram} target="_blank" rel="noopener noreferrer">Instagram</a><a href={contact.youtube} target="_blank" rel="noopener noreferrer">YouTube</a></span></div>
        <div><h2>Explore</h2><Link href="/services">Services</Link><Link href="/portfolio">Portfolio</Link><Link href="/about-us">About us</Link><Link href="/contact-us">Contact us</Link><a href="/account">My bookings</a></div>
      </div>
      <p className="f-foot__mark" aria-hidden="true">Dapper</p>
      <div className="f-foot__legal"><span>© 2026 Dapper Detailing Pros Ltd. • All Rights Reserved</span><span><a href="/privacy-policy">Privacy policy</a><a href="/terms-conditions">Terms &amp; conditions</a><a href="/refund-and-cancellation">Refund &amp; cancellation</a></span></div>
    </footer>
  );
}
