'use client';

import { useState } from 'react';
import styles from './owner.module.css';
import { DatePicker } from '../Pickers';

/* One date filter for every Master page: quick choices, "All time", or two dates of your own. */

export type RangePresetId = 'last7' | 'last30' | 'last90' | 'next7' | 'next30' | 'month' | 'year' | 'all' | 'custom';
export const RANGE_CHOICES: Record<string, string> = {
  last7: 'Last 7 days', last30: 'Last 30 days', last90: 'Last 90 days', next7: 'Next 7 days', next30: 'Next 30 days', month: 'This month', year: 'This year', all: 'All time', custom: 'Pick dates',
};
export type Custom = { from: string; to: string };

const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const ALL_TIME_START = '2018-01-01';
const MAX_DAYS = 3660;

export function resolveRange(id: RangePresetId, custom: Custom) {
  const today = new Date();
  const add = (n: number) => { const d = new Date(today); d.setDate(d.getDate() + n); return d; };
  switch (id) {
    case 'last7': return { from: ymd(add(-6)), to: ymd(today) };
    case 'last90': return { from: ymd(add(-89)), to: ymd(today) };
    case 'next7': return { from: ymd(today), to: ymd(add(6)) };
    case 'next30': return { from: ymd(today), to: ymd(add(29)) };
    case 'month': return { from: ymd(new Date(today.getFullYear(), today.getMonth(), 1)), to: ymd(new Date(today.getFullYear(), today.getMonth() + 1, 0)) };
    case 'year': return { from: ymd(new Date(today.getFullYear(), 0, 1)), to: ymd(today) };
    case 'all': return { from: ALL_TIME_START, to: ymd(today) };
    case 'custom': return custom;
    default: return { from: ymd(add(-29)), to: ymd(today) };
  }
}

export function customProblem(c: Custom): string {
  if (!c.from || !c.to) return 'Choose both dates.';
  if (c.to < c.from) return 'The end date comes before the start date.';
  if ((Date.parse(c.to) - Date.parse(c.from)) / 86400000 > MAX_DAYS) return 'Choose 10 years or less.';
  return '';
}

export default function RangeBar({ presets, preset, custom, onPreset, onCustom, label = 'Date range' }: {
  presets: RangePresetId[]; preset: RangePresetId; custom: Custom; onPreset: (p: RangePresetId) => void; onCustom: (c: Custom) => void; label?: string;
}) {
  // Edits to the two dates are held here and only applied when they make sense, so charts do not reload on half-typed dates.
  const [draft, setDraft] = useState<Custom>(custom);
  const problem = customProblem(draft);
  const change = (next: Custom) => { setDraft(next); if (!customProblem(next)) onCustom(next); };
  return (
    <>
      <div className={styles.segments} role="group" aria-label={label}>
        {[...presets, 'all' as const, 'custom' as const].map((id) => (
          <button key={id} type="button" className={styles.segment} aria-pressed={preset === id} onClick={() => onPreset(id)}>{RANGE_CHOICES[id]}</button>
        ))}
      </div>
      {preset === 'custom' && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <label className={styles.filterSelect}><span>From</span><DatePicker value={draft.from} max={draft.to || undefined} onChange={(e) => change({ ...draft, from: e.target.value })} /></label>
          <label className={styles.filterSelect}><span>To</span><DatePicker value={draft.to} min={draft.from || undefined} onChange={(e) => change({ ...draft, to: e.target.value })} /></label>
          {problem && <span role="alert" className="field-error">{problem}</span>}
        </div>
      )}
    </>
  );
}
