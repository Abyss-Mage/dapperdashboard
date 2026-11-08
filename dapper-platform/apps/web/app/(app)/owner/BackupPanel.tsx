'use client';

import { useState } from 'react';
import styles from './owner.module.css';

/* Whole-system backup: an encrypted file the owner keeps, and the way to bring the system back from one. The passphrase is never stored,
   so the file opens only with it, even after the server is gone. */

async function read<T>(r: Response): Promise<T> {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'That did not work. Please try again.');
  return b as T;
}

function size(bytes: number) {
  return bytes >= 1 << 30 ? `${(bytes / (1 << 30)).toFixed(1)} GB` : `${Math.max(1, Math.round(bytes / (1 << 20)))} MB`;
}

export default function BackupPanel({ organizationId }: { organizationId: string }) {
  const url = `/api/v1/organizations/${encodeURIComponent(organizationId)}/backups`;
  const [pass, setPass] = useState('');
  const [again, setAgain] = useState('');
  const [making, setMaking] = useState(false);
  const [madeNotice, setMadeNotice] = useState('');
  const [makeError, setMakeError] = useState('');

  const [file, setFile] = useState<File | null>(null);
  const [restorePass, setRestorePass] = useState('');
  const [confirm, setConfirm] = useState('');
  const [restoring, setRestoring] = useState(false);
  const [restoreNotice, setRestoreNotice] = useState('');
  const [restoreError, setRestoreError] = useState('');

  const mismatch = again !== '' && pass !== again;

  async function make(e: React.FormEvent) {
    e.preventDefault();
    setMaking(true); setMakeError(''); setMadeNotice('');
    try {
      const made = await read<{ id: string; sizeBytes: number }>(await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ passphrase: pass }) }));
      setMadeNotice(`Backup ready (${size(made.sizeBytes)}). Your download is starting. Keep the file and the passphrase in separate safe places.`);
      setPass(''); setAgain('');
      window.location.assign(`${url}/${made.id}`);
    } catch (err) { setMakeError((err as Error).message); } finally { setMaking(false); }
  }

  async function restore(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setRestoring(true); setRestoreError(''); setRestoreNotice('');
    // The server reads the fields before the file, so a wrong confirmation stops before the upload.
    const body = new FormData();
    body.append('passphrase', restorePass);
    body.append('confirm', confirm);
    body.append('file', file);
    try {
      const done = await read<{ backupCreatedAt: string; safetyCopy: string; keysDiffer: boolean }>(await fetch(`${url}/restore`, { method: 'POST', body }));
      setRestoreNotice(`Restored from the backup made ${new Date(done.backupCreatedAt).toLocaleString('en-CA')}. A safety copy of the data from just before is on the server as ${done.safetyCopy}, with the same passphrase.${done.keysDiffer ? ' This server uses different secret keys from the backup, so saved email and Square settings will not work until restored-keys.env, in the server’s backups folder, is copied into its .env.' : ''}`);
      setFile(null); setRestorePass(''); setConfirm('');
    } catch (err) { setRestoreError((err as Error).message); } finally { setRestoring(false); }
  }

  return (
    <div className={styles.stack}>
      <section className={styles.card} style={{ padding: 20 }}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>Download a backup</h2>
        <p className={styles.personSub}>One encrypted file with everything: bookings, invoices, customers, photos and documents. It protects you up to the moment you download it, so do this regularly and keep the file off this server.</p>
        <p className={styles.personSub}><strong>Nobody can recover a forgotten passphrase.</strong> Without it, the file can never be opened.</p>
        {makeError && <p role="alert" className="field-error">{makeError}</p>}
        {madeNotice && <p role="status" className={styles.status}>{madeNotice}</p>}
        <form onSubmit={make} autoComplete="off">
          <label className="form-field">Passphrase (at least 12 characters)<input type="password" value={pass} onChange={(e) => setPass(e.target.value)} disabled={making} maxLength={256} autoComplete="new-password" /></label>
          <label className="form-field">Passphrase again<input type="password" value={again} onChange={(e) => setAgain(e.target.value)} disabled={making} maxLength={256} autoComplete="new-password" /></label>
          {mismatch && <p role="alert" className="field-error">The two passphrases do not match.</p>}
          <button className="primary" disabled={making || pass.length < 12 || pass !== again}>{making ? 'Preparing, this can take a few minutes…' : 'Prepare and download'}</button>
        </form>
      </section>

      <section className={styles.card} style={{ padding: 20 }}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>Bring the system back from a backup</h2>
        <p className={styles.personSub}><strong>This replaces all current bookings, invoices, customers, photos and documents with what is in the file.</strong> Anything added since that backup is taken out of the system. A safety copy of today&rsquo;s data is saved on the server first. People may be signed out.</p>
        {restoreError && <p role="alert" className="field-error">{restoreError}</p>}
        {restoreNotice && <p role="status" className={styles.status}>{restoreNotice}</p>}
        <form onSubmit={restore} autoComplete="off">
          <label className="form-field">Backup file<input type="file" accept=".dapbk" onChange={(e) => setFile(e.target.files?.[0] ?? null)} disabled={restoring} /></label>
          <label className="form-field">The backup&rsquo;s passphrase<input type="password" value={restorePass} onChange={(e) => setRestorePass(e.target.value)} disabled={restoring} maxLength={256} autoComplete="off" /></label>
          <label className="form-field">Type RESTORE to confirm<input value={confirm} onChange={(e) => setConfirm(e.target.value)} disabled={restoring} autoComplete="off" /></label>
          <button className="primary" disabled={restoring || !file || restorePass.length < 12 || confirm !== 'RESTORE'}>{restoring ? 'Restoring, do not close this page…' : 'Replace everything with this backup'}</button>
        </form>
      </section>
    </div>
  );
}
