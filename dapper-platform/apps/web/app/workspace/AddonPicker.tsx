'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { Check, Plus, Search, X } from 'lucide-react';
import styles from './employee.module.css';

type Addon = { serviceId: string; name: string };

/* Add-on selection as a popup: a summary button with removable chips, and a glass sheet with search and large toggle rows.
   Native <dialog> gives focus handling, Escape to close and a backdrop. */
export default function AddonPicker({ serviceName, addons, picked, onChange }: { serviceName: string; addons: Addon[]; picked: string[]; onChange: (ids: string[]) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [query, setQuery] = useState('');
  const chosen = addons.filter((a) => picked.includes(a.serviceId));
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return addons.filter((a) => !q || a.name.toLowerCase().includes(q));
  }, [addons, query]);

  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    const reset = () => setQuery('');
    d.addEventListener('close', reset);
    return () => d.removeEventListener('close', reset);
  }, []);

  const toggle = (id: string) => onChange(picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]);

  return (
    <div className={styles.addonBlock}>
      <div className={styles.addonHead}>
        <div>
          <strong>Add-ons</strong>
          <span>Optional. They don’t change the appointment length.</span>
        </div>
        <button type="button" className={styles.addonOpen} onClick={() => dialog.current?.showModal()} aria-haspopup="dialog">
          <Plus size={16} aria-hidden="true" /> {chosen.length ? `${chosen.length} selected` : 'Choose add-ons'}
        </button>
      </div>
      {chosen.length > 0 && (
        <ul className={styles.addonChips} aria-label="Selected add-ons">
          {chosen.map((a) => (
            <li key={a.serviceId}>
              <span>{a.name}</span>
              <button type="button" onClick={() => toggle(a.serviceId)} aria-label={`Remove ${a.name}`}><X size={14} aria-hidden="true" /></button>
            </li>
          ))}
        </ul>
      )}

      <dialog ref={dialog} className={styles.addonDialog} aria-labelledby="addon-dialog-title" onClick={(e) => { if (e.target === dialog.current) dialog.current?.close(); }}>
        <div className={styles.addonSheet}>
          <header>
            <div>
              <h2 id="addon-dialog-title">Add-ons</h2>
              <p>{serviceName}</p>
            </div>
            <button type="button" className={styles.addonClose} onClick={() => dialog.current?.close()} aria-label="Close"><X size={18} aria-hidden="true" /></button>
          </header>
          {addons.length > 6 && (
            <label className={styles.addonSearch}>
              <Search size={16} aria-hidden="true" />
              <input type="search" placeholder="Search add-ons" aria-label="Search add-ons" value={query} onChange={(e) => setQuery(e.target.value)} />
            </label>
          )}
          <ul className={styles.addonList}>
            {visible.map((a) => {
              const on = picked.includes(a.serviceId);
              return (
                <li key={a.serviceId}>
                  <button type="button" role="checkbox" aria-checked={on} className={`${styles.addonRow} ${on ? styles.addonRowOn : ''}`} onClick={() => toggle(a.serviceId)}>
                    <span>{a.name}</span>
                    <span className={styles.addonTick} aria-hidden="true">{on && <Check size={16} />}</span>
                  </button>
                </li>
              );
            })}
            {visible.length === 0 && <li className={styles.addonNone}>No add-ons match.</li>}
          </ul>
          <footer>
            <button type="button" className={styles.addonClear} disabled={!picked.length} onClick={() => onChange([])}>Clear</button>
            <button type="button" className={styles.addonDone} onClick={() => dialog.current?.close()}>Done{chosen.length ? ` (${chosen.length})` : ''}</button>
          </footer>
        </div>
      </dialog>
    </div>
  );
}
