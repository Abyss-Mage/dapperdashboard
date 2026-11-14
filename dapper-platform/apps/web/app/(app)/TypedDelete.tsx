'use client';

import { useState } from 'react';
import FormPopup from './FormPopup';

/* Owner-only permanent delete, confirmed by typing a word shown in the dialog (error404 #116 #118 #124). The server checks the typed text
   and the owner's access again; this only keeps the button off until it matches. */
export default function TypedDelete({ title, explanation, word, wordLabel, url, onDone, onClose }: {
  title: string; explanation: string; word: string; wordLabel: string; url: string; onDone: () => void | Promise<void>; onClose: () => void;
}) {
  const [typed, setTyped] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState('');
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError('');
    try {
      const r = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ confirm: typed }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? 'It could not be deleted permanently.');
      await onDone();
    } catch (err) { setError(err instanceof Error ? err.message : 'It could not be deleted permanently.'); }
    finally { setBusy(false); }
  }
  return (
    <FormPopup label={title} onClose={() => { if (!busy) onClose(); }} width={520}>
      <form onSubmit={submit} style={{ display: 'grid', gap: 12 }}>
        <h3 style={{ margin: 0 }}>{title}</h3>
        <p style={{ margin: 0 }}>{explanation} It cannot be undone.</p>
        <label className="form-field" style={{ margin: 0 }}>Type {wordLabel} (<b style={{ overflowWrap: 'anywhere' }}>{word}</b>) to confirm
          <input value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" disabled={busy} autoFocus />
        </label>
        {error && <p role="alert" className="field-error" style={{ margin: 0 }}>{error}</p>}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button type="button" className="secondary" disabled={busy} onClick={onClose}>Keep it</button>
          <button className="primary" disabled={busy || typed.trim().toLowerCase() !== word.trim().toLowerCase()}>{busy ? 'Deleting…' : 'Delete permanently'}</button>
        </div>
      </form>
    </FormPopup>
  );
}
