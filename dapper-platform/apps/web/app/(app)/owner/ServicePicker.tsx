'use client';

import { useEffect, useMemo, useRef, useState } from 'react';

type Named = { id: string; name: string };

/* A button that shows what is chosen ("Every service", "3 services") and opens a popup to tick services, with a search box and
   Select all / Clear. Leaving everything unticked means every service. */
export default function ServicePicker({ services, value, onChange, disabled, label = 'Services' }: { services: Named[]; value: string[]; onChange: (ids: string[]) => void; disabled?: boolean; label?: string }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q ? services.filter((s) => s.name.toLowerCase().includes(q)) : services;
  }, [services, query]);
  const picked = new Set(value);
  const summary = value.length === 0 ? 'Every service' : value.length === 1 ? (services.find((s) => s.id === value[0])?.name ?? '1 service') : `${value.length} services`;
  const toggle = (id: string) => onChange(picked.has(id) ? value.filter((x) => x !== id) : [...value, id]);

  return (
    <div>
      <button type="button" className="secondary" disabled={disabled} onClick={() => { setQuery(''); setOpen(true); }} aria-haspopup="dialog" style={{ width: '100%', justifyContent: 'space-between', display: 'flex', alignItems: 'center', gap: 12, textAlign: 'left' }}>
        <span><span style={{ opacity: 0.7 }}>{label}: </span><strong>{summary}</strong></span>
        <span aria-hidden="true">▾</span>
      </button>
      <dialog ref={dialog} aria-label={label} onClose={() => setOpen(false)} onClick={(e) => { if (e.target === dialog.current) setOpen(false); }}
        style={{ width: 'min(520px, 94vw)', maxHeight: '82vh', padding: 0, border: '1px solid var(--glass-line, #3a3a42)', borderRadius: 18, background: 'var(--surface, #17171b)', color: 'var(--text, inherit)' }}>
        <div style={{ display: 'grid', gap: 12, padding: 18 }}>
          <strong style={{ fontSize: 17 }}>{label}</strong>
          <input type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search services" aria-label="Search services" style={{ width: '100%' }} />
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button type="button" className="secondary" onClick={() => onChange([...new Set([...value, ...shown.map((s) => s.id)])])}>Select all{query ? ' shown' : ''}</button>
            <button type="button" className="secondary" onClick={() => onChange([])}>Clear (every service)</button>
          </div>
          <div style={{ maxHeight: '44vh', overflowY: 'auto', display: 'grid', gap: 2, border: '1px solid var(--glass-line, #3a3a42)', borderRadius: 12, padding: 6 }}>
            {shown.length === 0 && <p style={{ margin: 8 }}>No service matches.</p>}
            {shown.map((s) => (
              <label key={s.id} style={{ display: 'flex', gap: 10, alignItems: 'center', padding: '7px 8px', borderRadius: 8, cursor: 'pointer' }}>
                <input type="checkbox" checked={picked.has(s.id)} onChange={() => toggle(s.id)} />
                <span>{s.name}</span>
              </label>
            ))}
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ opacity: 0.7 }}>{value.length === 0 ? 'Nothing ticked: every service' : `${value.length} ticked`}</span>
            <button type="button" className="primary" onClick={() => setOpen(false)}>Done</button>
          </div>
        </div>
      </dialog>
    </div>
  );
}
