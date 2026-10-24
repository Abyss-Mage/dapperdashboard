'use client';

import { useState, type ReactNode } from 'react';
import styles from './owner.module.css';

export type SubPage = { id: string; label: string; show?: boolean; node: () => ReactNode };

/* A Master section split into small pages. Only the page you pick is drawn, so each screen holds one job. */
export default function SubPages({ pages, label }: { pages: SubPage[]; label: string }) {
  const shown = pages.filter((p) => p.show !== false);
  const [current, setCurrent] = useState(shown[0]?.id ?? '');
  const active = shown.find((p) => p.id === current) ?? shown[0];
  if (!active) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {shown.length > 1 && (
        <div className={styles.segments} role="group" aria-label={label}>
          {shown.map((p) => (
            <button key={p.id} type="button" className={styles.segment} aria-pressed={active.id === p.id} onClick={() => setCurrent(p.id)}>{p.label}</button>
          ))}
        </div>
      )}
      {active.node()}
    </div>
  );
}
