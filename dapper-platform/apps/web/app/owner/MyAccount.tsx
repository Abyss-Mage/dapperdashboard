'use client';

import { useEffect, useState } from 'react';
import styles from './owner.module.css';
import { MfaManager, ProfileForm, type Profile } from '../workspace/AccountSecurity';

/* The signed-in person's own account in Master: name and phone, and two-factor sign-in. Roles and franchise access are assigned in
   People & Access, so nothing here changes permissions. */
export default function MyAccount() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loaded, setLoaded] = useState(false);
  useEffect(() => {
    let live = true;
    fetch('/api/v1/account/profile', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((p) => { if (live) setProfile(p); })
      .catch(() => undefined)
      .finally(() => { if (live) setLoaded(true); });
    return () => { live = false; };
  }, []);

  return (
    <div className={styles.stack}>
      <section className={styles.resource}>
        <h3>{profile?.fullName?.trim() || 'Your account'}</h3>
        <p className={styles.personSub}>{profile?.email ?? (loaded ? 'Signed in' : 'Loading…')}</p>
        <p className={styles.footnote}>Your roles and the franchises you can reach are managed in People &amp; Access.</p>
      </section>
      <section className={styles.resource}>
        <h3>Your details</h3>
        <ProfileForm initial={profile} onSaved={setProfile} />
      </section>
      <section className={styles.resource}>
        <h3>Two-factor sign-in</h3>
        <MfaManager />
      </section>
    </div>
  );
}
