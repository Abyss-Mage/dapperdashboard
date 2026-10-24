'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

export type Role = { id: string; name: string; description: string; version: number; capabilities: string[]; assignmentCount: number };

// Display grouping only. The server's capability catalogue is the source of truth for what exists.
const GROUPS: { label: string; caps: string[] }[] = [
  { label: 'Customers & vehicles', caps: ['customers.read', 'customers.create', 'customer_accounts.manage', 'vehicles.read', 'vehicles.create'] },
  { label: 'Bookings & scheduling', caps: ['bookings.read', 'bookings.create', 'bookings.manage', 'scheduling.read', 'scheduling.manage', 'scheduling.reserve', 'scheduling.release'] },
  { label: 'Work orders', caps: ['jobs.read', 'jobs.create', 'jobs.complete', 'jobs.contribute'] },
  { label: 'Quotes', caps: ['quotes.read', 'quotes.create', 'quotes.manage', 'quotes.issue'] },
  { label: 'Invoices & payments', caps: ['invoices.read', 'invoices.create', 'invoices.issue', 'invoices.manage', 'invoices.configure', 'payments.read', 'payments.record'] },
  { label: 'Staff time & location', caps: ['attendance.record', 'attendance.read', 'attendance.manage'] },
  { label: 'Documents', caps: ['sop.read', 'sop.acknowledge', 'sop.manage'] },
  { label: 'Partner dealerships', caps: ['dealership.read', 'dealership.manage'] },
  { label: 'Catalogue, marketing & content', caps: ['catalogue.read', 'catalogue.manage', 'marketing.read', 'marketing.manage', 'content.manage'] },
  { label: 'Organization settings & people', caps: ['structure.read', 'structure.manage', 'access.manage', 'roles.manage', 'roles.assign'] },
];
// Only meaningful when a role is assigned organization-wide.
const ORG_ONLY = new Set(['structure.read', 'structure.manage', 'invoices.configure', 'access.manage', 'roles.manage', 'roles.assign']);
const label = (cap: string) => cap.split('.')[1].replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase());

async function readJson(r: Response) {
  if (r.status === 204) return null;
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

export default function RoleManager({ organizationId, onChanged }: { organizationId: string; onChanged: () => void }) {
  const base = `/api/v1/organizations/${organizationId}`;
  const [roles, setRoles] = useState<Role[] | null>(null);
  const [known, setKnown] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<Role | 'new' | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [confirm, setConfirm] = useState<Role | null>(null);

  const load = useCallback(async () => {
    try {
      const [r, c] = await Promise.all([fetch(`${base}/roles`, { cache: 'no-store' }).then(readJson), fetch(`${base}/capabilities`, { cache: 'no-store' }).then(readJson)]);
      setRoles(r);
      setKnown(new Set(c.allCapabilities));
    } catch (e) {
      setRoles(null);
      setError(e instanceof Error ? e.message : 'Roles could not be loaded.');
    }
  }, [base]);
  useEffect(() => {
    void load();
  }, [load]);

  const groups = useMemo(() => {
    const listed = new Set(GROUPS.flatMap((g) => g.caps));
    const extra = [...known].filter((c) => !listed.has(c)).sort();
    return extra.length ? [...GROUPS, { label: 'Other', caps: extra }] : GROUPS;
  }, [known]);

  function open(role: Role | 'new') {
    setEditing(role);
    setError('');
    setNotice('');
    setName(role === 'new' ? '' : role.name);
    setDescription(role === 'new' ? '' : role.description);
    setPicked(new Set(role === 'new' ? [] : role.capabilities));
  }
  const toggle = (cap: string) => setPicked((cur) => { const n = new Set(cur); n.has(cap) ? n.delete(cap) : n.add(cap); return n; });
  const toggleGroup = (caps: string[], on: boolean) => setPicked((cur) => { const n = new Set(cur); caps.filter((c) => known.has(c)).forEach((c) => (on ? n.add(c) : n.delete(c))); return n; });

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!picked.size) return setError('Choose at least one permission.');
    setBusy(true);
    setError('');
    try {
      const payload = { name: name.trim(), description: description.trim(), capabilities: [...picked].sort() };
      const isNew = editing === 'new';
      await readJson(await fetch(isNew ? `${base}/roles` : `${base}/roles/${(editing as Role).id}`, { method: isNew ? 'POST' : 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(isNew ? payload : { ...payload, version: (editing as Role).version }) }));
      setNotice(isNew ? `Role "${payload.name}" created.` : `Role "${payload.name}" updated. People holding it get the change immediately.`);
      setEditing(null);
      await load();
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The role could not be saved.');
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!confirm) return;
    setBusy(true);
    setError('');
    try {
      await readJson(await fetch(`${base}/roles/${confirm.id}?version=${confirm.version}`, { method: 'DELETE' }));
      setNotice(`Role "${confirm.name}" deleted.`);
      setConfirm(null);
      await load();
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The role could not be deleted.');
      setConfirm(null);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <p className={styles.personSub}>A role is a named set of permissions. Assign it to a person for the whole organization, one franchise or one location under “Assign roles”.</p>
        {!editing && <button type="button" className="primary" onClick={() => open('new')}>New role</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'New role' : `Edit “${editing.name}”`}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Name<input required minLength={2} maxLength={80} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} /></label>
            <label className="form-field" style={{ flexBasis: 320 }}>Description (optional)<input maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} disabled={busy} /></label>
          </div>
          <div className={styles.capGroups}>
            {groups.map((g) => {
              const caps = g.caps.filter((c) => known.has(c));
              if (!caps.length) return null;
              const all = caps.every((c) => picked.has(c));
              return (
                <fieldset key={g.label} className={styles.capGroup} disabled={busy}>
                  <legend>{g.label}</legend>
                  <button type="button" className={styles.linkBtn} onClick={() => toggleGroup(caps, !all)}>{all ? 'Clear group' : 'Select all'}</button>
                  {caps.map((c) => (
                    <label key={c} className={styles.capItem}>
                      <input type="checkbox" checked={picked.has(c)} onChange={() => toggle(c)} />
                      <span>{label(c)}</span>
                      <code>{c}</code>
                      {ORG_ONLY.has(c) && <em>organization-wide only</em>}
                    </label>
                  ))}
                </fieldset>
              );
            })}
          </div>
          <p className={styles.footnote}>{picked.size} permission{picked.size === 1 ? '' : 's'} selected. Permissions marked organization-wide only take effect when the role is assigned to someone for the whole organization. Roles you create are your definitions, not a client-approved template.</p>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2 || !picked.size}>{busy ? 'Saving…' : editing === 'new' ? 'Create role' : 'Save role'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </form>
      )}

      {!roles && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading roles" />}
      {roles?.length === 0 && <p className={styles.status}>No roles yet.</p>}
      {roles?.map((r) => (
        <section key={r.id} className={styles.resource}>
          <div className={styles.resourceHead}>
            <div>
              <h3>{r.name}</h3>
              <p className={styles.personSub}>{r.description || 'No description'}</p>
            </div>
            <span className={styles.badge}>{r.capabilities.length} permissions · {r.assignmentCount} assigned</span>
          </div>
          <p className={styles.personSub}>{GROUPS.map((g) => { const n = g.caps.filter((c) => r.capabilities.includes(c)).length; return n ? `${g.label} ${n}/${g.caps.length}` : null; }).filter(Boolean).join(' · ')}</p>
          <div className={styles.rowBetween}>
            <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(r)}>Edit</button>
            <button type="button" className="secondary" disabled={busy || !!editing || r.assignmentCount > 0} title={r.assignmentCount > 0 ? 'Revoke its assignments first' : undefined} onClick={() => setConfirm(r)}>Delete</button>
          </div>
        </section>
      ))}

      <ConfirmDialog isOpen={confirm !== null} title="Delete role" entityName={confirm?.name ?? ''} entityType="Role" actionType="delete" isBusy={busy}
        explanation="The role definition is removed. A role that is assigned to anyone cannot be deleted, so revoke its assignments first. This cannot be undone."
        impactNotice="Nobody holds this role right now." confirmLabel="Delete role" onConfirm={remove} onCancel={() => { if (!busy) setConfirm(null); }} />
    </div>
  );
}
