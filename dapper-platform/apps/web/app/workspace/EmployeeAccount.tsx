'use client';
import { LogOut, MapPin, ShieldCheck } from 'lucide-react';
import { useEffect, useState } from 'react';
import ThemeToggle from '../ThemeToggle';
import { MfaManager, ProfileForm, type Profile } from './AccountSecurity';
import styles from './employee.module.css';

type Site = { locationName: string; dealershipName: string; timezone: string };
const capabilityLabels: Record<string, string> = {
  'attendance.record': 'Clock in and out, take breaks',
  'attendance.read': 'View attendance records',
  'bookings.read': 'See assigned bookings',
  'bookings.create': 'Create walk-in bookings',
  'catalogue.read': 'See services',
  'payments.record': 'Take cash and card payments',
  'invoices.read': 'See invoices for your bookings',
};

/* Read-only account summary. Access is assigned by an administrator in Master; nothing here changes permissions.
   Password changes are not offered because the application has no self-service password endpoint yet. */
export default function EmployeeAccount({ employee, site, actions, busy, onSignOut }: { employee: string; site: Site; actions: string[]; busy: boolean; onSignOut: () => void }) {
  const initial = (employee.trim()[0] ?? '?').toUpperCase();
  const [profile, setProfile] = useState<Profile | null>(null);
  useEffect(() => { let live = true; fetch('/api/v1/account/profile', { cache: 'no-store' }).then(r => r.ok ? r.json() : null).then(p => { if (live) setProfile(p); }).catch(() => undefined); return () => { live = false; }; }, []);
  return <div className={styles.accountGrid}>
    <section className={styles.uploadCard} aria-labelledby="account-title">
      <div className={styles.accountHead}><span className={styles.avatar} aria-hidden="true">{initial}</span><div><h2 id="account-title">{employee || 'Your account'}</h2><p className={styles.help}>Signed in with your Dapper work account.</p></div></div>
      <dl className={styles.facts}>
        <div><dt><MapPin size={16} aria-hidden="true" />Location</dt><dd>{site.locationName}</dd></div>
        <div><dt>Timezone</dt><dd>{site.timezone}</dd></div>
      </dl>
      <div className={styles.accountActions}>
        <div className={styles.themeRow}><span>Appearance</span><ThemeToggle className={styles.iconButton} compact /></div>
        <button className={styles.secondary} onClick={onSignOut} disabled={busy}><LogOut size={18} aria-hidden="true" />Sign out</button>
      </div>
      <p className={styles.help}>Your worksite and access are set by your administrator. To change your password, name or worksite, contact them.</p>
    </section>
    <aside className={styles.recentCard} aria-labelledby="access-title">
      <div className={styles.sectionTop}><h2 id="access-title"><ShieldCheck size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />What you can do</h2><span className={styles.count}>{actions.length}</span></div>
      {actions.length ? <ul className={styles.recentList}>{actions.map(a => <li key={a}><strong>{capabilityLabels[a] ?? a}</strong></li>)}</ul> : <p className={styles.emptyNote}>No access has been assigned yet.</p>}
    </aside>
    <section className={styles.uploadCard} aria-labelledby="details-title"><div className={styles.sectionTop}><h2 id="details-title">Your details</h2></div><ProfileForm initial={profile} onSaved={setProfile} /></section>
    <section className={styles.recentCard} aria-labelledby="security-title"><div className={styles.sectionTop}><h2 id="security-title">Two-factor authentication</h2></div><MfaManager /></section>
  </div>;
}
