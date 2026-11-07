'use client';

import { useRef, useState } from 'react';
import p from './profile.module.css';

export type SavedAddress = { id: string; label: string; address: string; isDefault: boolean };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

// A square picture, centred and cropped, small enough to store (about 384 px, JPEG).
async function squarePicture(file: File): Promise<Blob> {
  const bitmap = await createImageBitmap(file);
  const side = Math.min(bitmap.width, bitmap.height), size = 384;
  const canvas = document.createElement('canvas');
  canvas.width = canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Your browser could not prepare the picture.');
  ctx.drawImage(bitmap, (bitmap.width - side) / 2, (bitmap.height - side) / 2, side, side, 0, 0, size, size);
  return new Promise((resolve, reject) => canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('The picture could not be prepared.'))), 'image/jpeg', 0.88));
}

export function Avatar({ customerId, name, has, version = 0, size = 96 }: { customerId: string; name: string; has: boolean; version?: number; size?: number }) {
  const initials = name.trim().split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('') || '?';
  return (
    <span className={p.avatar} style={{ width: size, height: size, fontSize: size * 0.36 }} aria-hidden="true">
      {has ? /* eslint-disable-next-line @next/next/no-img-element */ <img src={`/api/v1/portal/accounts/${encodeURIComponent(customerId)}/avatar?v=${version}`} alt="" width={size} height={size} /> : initials}
    </span>
  );
}

/* The customer's own profile picture and saved addresses. Everything goes through their account link; the default address is also the one our
   team sees on their record for mobile jobs, and the booking pre-fills from it. */
export default function AccountProfile({ customerId, name, email, hasAvatar, addresses, onChanged }: { customerId: string; name: string; email?: string; hasAvatar: boolean; addresses: SavedAddress[]; onChanged: () => void }) {
  const root = `/api/v1/portal/accounts/${encodeURIComponent(customerId)}`;
  const pick = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [version, setVersion] = useState(1);
  const [editing, setEditing] = useState<SavedAddress | 'new' | null>(null);
  const [label, setLabel] = useState(''), [address, setAddress] = useState('');

  async function run(fn: () => Promise<string>) {
    setBusy(true); setError(''); setNotice('');
    try { setNotice(await fn()); onChanged(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }
  async function upload(files: FileList | null) {
    const file = files?.[0];
    if (pick.current) pick.current.value = '';
    if (!file) return;
    if (!/^image\//.test(file.type)) { setError('Choose a picture (JPEG or PNG).'); return; }
    await run(async () => {
      const blob = await squarePicture(file);
      await readJson(await fetch(`${root}/avatar`, { method: 'PUT', headers: { 'Content-Type': 'image/jpeg' }, body: blob }));
      setVersion((v) => v + 1);
      return 'Your picture was updated.';
    });
  }
  const removePicture = () => run(async () => { await readJson(await fetch(`${root}/avatar`, { method: 'DELETE' })); setVersion((v) => v + 1); return 'Your picture was removed.'; });

  function open(next: SavedAddress | 'new') { setEditing(next); setLabel(next === 'new' ? '' : next.label); setAddress(next === 'new' ? '' : next.address); setError(''); setNotice(''); }
  const saveAddress = (e: React.FormEvent) => {
    e.preventDefault();
    const was = editing;
    void run(async () => {
      await readJson(await fetch(`${root}/addresses`, send('PUT', { id: was && was !== 'new' ? was.id : '', label, address, isDefault: was && was !== 'new' ? was.isDefault : false })));
      setEditing(null);
      return 'Address saved.';
    });
  };
  const makeDefault = (a: SavedAddress) => run(async () => { await readJson(await fetch(`${root}/addresses`, send('PUT', { ...a, isDefault: true }))); return `${a.label} is now your default address.`; });
  const remove = (a: SavedAddress) => run(async () => { await readJson(await fetch(`${root}/addresses/${a.id}`, { method: 'DELETE' })); return `${a.label} was removed.`; });

  return (
    <div className={p.wrap}>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={p.notice}>{notice}</p>}
      <section className={p.card} aria-labelledby="pf-photo">
        <Avatar customerId={customerId} name={name} has={hasAvatar} version={version} size={96} />
        <div className={p.who}>
          <h3 id="pf-photo">{name}</h3>
          {email && <p>{email}</p>}
          <div className={p.row}>
            <input ref={pick} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={(e) => void upload(e.target.files)} />
            <button type="button" className={p.btn} disabled={busy} onClick={() => pick.current?.click()}>{hasAvatar ? 'Change photo' : 'Add a photo'}</button>
            {hasAvatar && <button type="button" className={p.ghost} disabled={busy} onClick={() => void removePicture()}>Remove</button>}
          </div>
        </div>
      </section>
      <section className={p.card + ' ' + p.col} aria-labelledby="pf-addr">
        <div className={p.head}>
          <h3 id="pf-addr">Saved addresses</h3>
          {!editing && addresses.length < 10 && <button type="button" className={p.btn} disabled={busy} onClick={() => open('new')}>Add an address</button>}
        </div>
        <p className={p.sub}>Used to fill in your booking when you ask us to come to you. Your default is the one our team sees on your record.</p>
        {addresses.length === 0 && !editing && <p className={p.empty}>No saved addresses yet.</p>}
        <ul className={p.list}>
          {addresses.map((a) => (
            <li key={a.id} className={p.item}>
              <div><strong>{a.label}</strong>{a.isDefault && <span className={p.tag}>Default</span>}<p>{a.address}</p></div>
              <div className={p.row}>
                {!a.isDefault && <button type="button" className={p.ghost} disabled={busy} onClick={() => void makeDefault(a)}>Make default</button>}
                <button type="button" className={p.ghost} disabled={busy || !!editing} onClick={() => open(a)}>Edit</button>
                <button type="button" className={p.ghost} disabled={busy} onClick={() => void remove(a)}>Remove</button>
              </div>
            </li>
          ))}
        </ul>
        {editing && (
          <form onSubmit={saveAddress} className={p.form}>
            <label className="form-field">Name for this address<input required maxLength={40} value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Home, Work, Cottage" disabled={busy} /></label>
            <label className="form-field">Full address<input required minLength={5} maxLength={300} value={address} onChange={(e) => setAddress(e.target.value)} autoComplete="street-address" placeholder="Street, city, postal code" disabled={busy} /></label>
            <div className={p.row}>
              <button className={p.btn} disabled={busy}>{busy ? 'Saving…' : 'Save address'}</button>
              <button type="button" className={p.ghost} disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
            </div>
          </form>
        )}
      </section>
    </div>
  );
}
