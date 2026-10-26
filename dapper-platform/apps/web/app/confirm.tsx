'use client';

import { useEffect, useState } from 'react';
import ConfirmDialog from './workspace/ConfirmDialog';

/* A nicer "Are you sure?" than the browser's own box: await askConfirm({...}) anywhere in client code and get true or false.
   <ConfirmHost /> (mounted once in the layout) draws it with the glass dialog. */

type Ask = { title: string; message: string; detail?: string; confirmLabel?: string; tone?: 'danger' | 'default' };
type Pending = Ask & { resolve: (answer: boolean) => void };
let push: ((p: Pending) => void) | null = null;

export function askConfirm(ask: Ask): Promise<boolean> {
  return new Promise((resolve) => {
    if (!push) { resolve(window.confirm(ask.message)); return; } // only if the host is not on the page
    push({ ...ask, resolve });
  });
}

export function ConfirmHost() {
  const [pending, setPending] = useState<Pending | null>(null);
  const [shown, setShown] = useState<Ask | null>(null);
  useEffect(() => { push = (p) => { setShown(p); setPending(p); }; return () => { push = null; }; }, []);
  const answer = (value: boolean) => { pending?.resolve(value); setPending(null); };
  return (
    <ConfirmDialog
      isOpen={pending !== null} actionType="confirm" title={shown?.title ?? ''} message={shown?.message} explanation={shown?.detail}
      confirmLabel={shown?.confirmLabel} tone={shown?.tone} onConfirm={() => answer(true)} onCancel={() => answer(false)}
    />
  );
}
