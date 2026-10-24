import ThemeToggle from '../ThemeToggle';
import {CalendarDays, ShieldCheck} from 'lucide-react';

export default function PublicBookingPortal() {
  return (
    <main className="secure-workspace" style={{maxWidth: 900, margin: '0 auto', padding: '1.5rem'}}>
      <header style={{display: 'flex', flexWrap: 'wrap', gap: '1rem', justifyContent: 'space-between', alignItems: 'center', marginBottom: '2rem'}}>
        <a href="/" className="brand" style={{textDecoration: 'none'}}>
          <span className="brandmark">D<span>↗</span></span>
          <span>DAPPER<small>DETAILING PROS</small></span>
        </a>
        <ThemeToggle />
      </header>
      <section className="panel detail-card" aria-labelledby="booking-heading">
        <CalendarDays size={28} aria-hidden="true" />
        <h1 id="booking-heading">Your next Dapper appointment</h1>
        <p>Sign in to choose your vehicle, explore published service offers and request an available time through your customer account.</p>
        <p><ShieldCheck size={18} aria-hidden="true" /> Your account shows only your own vehicles, appointment requests and invoices.</p>
        <p>Available offers show their price or quote requirement, duration, deposit and cancellation terms before you send a request. A request is not a confirmed appointment; its status is shown in your account.</p>
        <a className="primary text-button" href="/account">Sign in &amp; find a time</a>
        <p style={{marginTop:'1rem'}}>New to Dapper or no available offers? <a href="/contact-us">Contact the team</a> to arrange your service and account access.</p>
      </section>
    </main>
  );
}
