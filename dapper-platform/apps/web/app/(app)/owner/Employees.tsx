'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import Select from '../GlassSelect';

type Person = {
  principalId: string; email: string; active: boolean; fullName: string; phone: string; profileComplete: boolean; administrator: boolean;
  locationName: string; locationCode: string; roles: string[]; scope: string; lastSignIn: string | null; documentsTotal: number; documentsAcknowledged: number;
  onboardingStatus: 'complete' | 'incomplete' | 'not_assigned' | 'inactive'; missing: string[];
};
type Role = { id: string; name: string };
type Assignment = { id: string; principalId: string; roleName: string; scopeType: 'location' | 'franchise' | 'organization'; scopeName: string; version: number };
type Named = { id: string; name: string };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The change could not be saved.'));
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
const when = (iso: string | null) => (iso ? new Date(iso).toLocaleString('en-CA', { dateStyle: 'medium', timeStyle: 'short' }) : 'Never');
const MISSING: Record<string, string> = { details: 'name and phone', documents: 'SOP documents', franchise: 'franchise assignment' };

/* The employees page: everyone in the organization with their roles, franchise and last sign-in; filter by role or status, open a
   person to change their roles, and terminate or reinstate them. Termination blocks sign-in at once and keeps their history. */
/* Only people whose role is Employee are listed here; owners and managers are managed under Team. */
const isEmployee = (p: { roles: string[] }) => p.roles.some((r) => r.trim().toLowerCase() === 'employee');

export default function Employees({ organizationId, franchiseName = '', onChanged }: { organizationId: string; franchiseName?: string; onChanged?: () => void }) {
  const base = `/api/v1/organizations/${organizationId}`;
  const [people, setPeople] = useState<Person[] | null>(null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [assignments, setAssignments] = useState<Assignment[]>([]);
  const [franchises, setFranchises] = useState<Named[]>([]);
  const [query, setQuery] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [status, setStatus] = useState('');
  const [franchise, setFranchise] = useState(franchiseName);
  // Follow the franchise picked in the sidebar (error404 #53): switching franchise shows that franchise's people.
  useEffect(() => { setFranchise(franchiseName); }, [franchiseName]);
  const [open, setOpen] = useState<Person | null>(null);
  const [newRole, setNewRole] = useState('');
  const [newScope, setNewScope] = useState('organization');
  const [confirm, setConfirm] = useState<{ person: Person; action: 'terminate' | 'reinstate' | 'remove' } | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    try {
      const [p, r, a, st] = await Promise.all(['access/people', 'roles', 'role-assignments', 'structure'].map((x) => fetch(`${base}/${x}`, { cache: 'no-store' }).then(readJson)));
      setPeople(((p.people ?? []) as Person[]).filter(isEmployee));
      setRoles(r);
      setAssignments(a);
      setFranchises((st.franchises ?? []).filter((f: { franchiseId: string | null }) => !!f.franchiseId).map((f: { franchiseId: string; name: string; code: string }) => ({ id: f.franchiseId, name: f.code ? `${f.code} · ${f.name}` : f.name })));
      setNewRole((c) => c || r[0]?.id || '');
      setError('');
    } catch (e) { setPeople(null); setError(e instanceof Error ? e.message : 'Employees could not be loaded.'); }
  }, [base]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { if (open && people) setOpen(people.find((p) => p.principalId === open.principalId) ?? null); }, [people]); // eslint-disable-line react-hooks/exhaustive-deps

  const roleNames = useMemo(() => [...new Set((people ?? []).flatMap((p) => p.roles))].sort(), [people]);
  const franchiseNames = useMemo(() => [...new Set((people ?? []).map((p) => p.locationName).filter(Boolean))].sort(), [people]);
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (people ?? []).filter((p) =>
      (!q || [p.fullName, p.email, p.phone].some((v) => (v ?? '').toLowerCase().includes(q))) &&
      (!franchise || p.locationName === franchise) &&
      (!roleFilter || p.roles.includes(roleFilter)) &&
      (!status || (status === 'terminated' ? !p.active : status === 'attention' ? p.active && p.missing.length > 0 : p.active)));
  }, [people, query, roleFilter, status, franchise]);

  async function run(fn: () => Promise<string>) {
    setBusy(true); setError(''); setNotice('');
    try { setNotice(await fn()); await load(); onChanged?.(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }
  const assign = (p: Person) => run(async () => {
    await readJson(await fetch(`${base}/role-assignments`, send('POST', { principalId: p.principalId, roleId: newRole, scopeType: newScope === 'organization' ? 'organization' : 'franchise', scopeId: newScope === 'organization' ? organizationId : newScope })));
    return 'Role added.';
  });
  const revoke = (a: Assignment) => run(async () => { await readJson(await fetch(`${base}/role-assignments/${a.id}?version=${a.version}`, send('DELETE'))); return `${a.roleName} removed.`; });
  const setActive = (p: Person, action: 'terminate' | 'reinstate' | 'remove') => run(async () => {
    await readJson(await fetch(`${base}/members/${p.principalId}/${action}`, send('POST', {})));
    setConfirm(null);
    if (action === 'remove') setOpen(null);
    return action === 'terminate' ? `${p.fullName || p.email} was terminated and signed out.` : action === 'remove' ? `${p.fullName || p.email} was removed from the team.` : `${p.fullName || p.email} was reinstated.`;
  });

  const mine = open ? assignments.filter((a) => a.principalId === open.principalId) : [];
  const scopeLabel = (a: Assignment) => (a.scopeType === 'organization' ? 'Whole organization' : a.scopeName);

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Employees</h2>
        <button type="button" className="secondary" disabled={busy} onClick={() => void load()}>Refresh</button>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.custFilters}>
        <input type="search" className={styles.searchBox} placeholder="Search name, email or phone" aria-label="Search employees" value={query} onChange={(e) => setQuery(e.target.value)} />
        <Select value={franchise} onChange={(e) => setFranchise(e.target.value)} aria-label="Franchise"><option value="">All franchises</option>{[...new Set([franchiseName, ...franchiseNames])].filter(Boolean).map((f) => <option key={f} value={f}>{f}</option>)}</Select>
        <Select value={roleFilter} onChange={(e) => setRoleFilter(e.target.value)} aria-label="Role"><option value="">Any role</option>{roleNames.map((r) => <option key={r} value={r}>{r}</option>)}</Select>
        <Select value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Status"><option value="">Everyone</option><option value="active">Active</option><option value="attention">Needs attention</option><option value="terminated">Terminated</option></Select>
        {(query || roleFilter || status || franchise) && <button type="button" className="secondary" onClick={() => { setQuery(''); setRoleFilter(''); setStatus(''); setFranchise(''); }}>Clear filters</button>}
      </div>
      {!people && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading employees" />}
      {people && people.length === 0 && <p className={styles.status}>No employees yet. Invite someone under Team and give them the Employee role. They sign in to the workspace only.</p>}
      {people && people.length > 0 && shown.length === 0 && <p className={styles.status}>{franchise && !query && !roleFilter && !status ? `No employees at ${franchise} yet.` : 'No one matches these filters.'}</p>}
      <ul className={styles.people}>
        {shown.map((p) => (
          <li key={p.principalId} className={styles.person} style={{ opacity: p.active ? 1 : 0.65, gridTemplateColumns: 'minmax(0,1fr) auto', alignItems: 'center' }}>
            <button type="button" className={styles.personButton} onClick={() => { setOpen(p); setNotice(''); }} aria-label={`Open ${p.fullName || p.email}`} style={{ width: '100%' }}>
              <span className={styles.custRow}>
                <strong className={styles.custName}>{p.fullName || p.email}</strong>
                <span className={styles.custContact}>{p.phone || 'No phone'}</span>
                <span className={styles.custContact}>{p.email}</span>
                <span className={styles.custCount}>{p.active ? 'Active' : 'Terminated'}</span>
                <span className={styles.custAddress}>{[p.roles.length ? p.roles.join(', ') : 'No role', p.locationName || p.scope || 'No franchise', `Last sign-in: ${when(p.lastSignIn)}`].join(' · ')}</span>
              </span>
            </button>
            <div className={styles.svcActions}>
              <button type="button" className="secondary" disabled={busy} onClick={() => { setOpen(p); setNotice(''); }}>Manage</button>
              {p.active
                ? <button type="button" className="secondary" disabled={busy} onClick={() => setConfirm({ person: p, action: 'terminate' })}>Terminate</button>
                : <button type="button" className="secondary" disabled={busy} onClick={() => setConfirm({ person: p, action: 'reinstate' })}>Reinstate</button>}
            </div>
          </li>
        ))}
      </ul>

      {open && <>
        <button type="button" className={styles.drawerScrim} aria-label="Close employee details" onClick={() => setOpen(null)} />
        <aside data-motion="drawer" className={styles.drawer} role="dialog" aria-modal="true" aria-label={`${open.fullName || open.email}, employee details`}>
          <div className={styles.rowBetween}>
            <div>
              <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>{open.fullName || open.email}</h2>
              <p className={styles.personSub}>{open.active ? 'Active' : 'Terminated'} · last sign-in {when(open.lastSignIn)}</p>
            </div>
            <button type="button" className="secondary" onClick={() => setOpen(null)}>Close</button>
          </div>
          <dl style={{ display: 'grid', gridTemplateColumns: '130px 1fr', gap: '6px 12px', margin: '16px 0' }}>
            <dt className={styles.personSub}>Email</dt><dd style={{ margin: 0, fontWeight: 600 }}>{open.email}</dd>
            <dt className={styles.personSub}>Phone</dt><dd style={{ margin: 0, fontWeight: 600 }}>{open.phone || 'Not given'}</dd>
            <dt className={styles.personSub}>Franchise</dt><dd style={{ margin: 0, fontWeight: 600 }}>{open.locationName ? `${open.locationCode ? open.locationCode + ' · ' : ''}${open.locationName}` : open.scope || 'Not assigned'}</dd>
            <dt className={styles.personSub}>SOP documents</dt><dd style={{ margin: 0, fontWeight: 600 }}>{open.documentsAcknowledged} of {open.documentsTotal} read</dd>
            {open.missing.length > 0 && <><dt className={styles.personSub}>Waiting on</dt><dd style={{ margin: 0, fontWeight: 600 }}>{open.missing.map((m) => MISSING[m] ?? m).join(', ')}</dd></>}
          </dl>

          <h3 style={{ fontSize: 15, margin: '0 0 8px' }}>Roles</h3>
          {mine.length === 0 ? <p className={styles.personSub}>No roles yet.</p> : (
            <ul className={styles.people} style={{ gap: 6 }}>
              {mine.map((a) => (
                <li key={a.id} className={styles.rowBetween} style={{ alignItems: 'center' }}>
                  <span><strong>{a.roleName}</strong> <span className={styles.personSub}>· {scopeLabel(a)}</span></span>
                  <button type="button" className="secondary" disabled={busy || !open.active} onClick={() => void revoke(a)}>Remove</button>
                </li>
              ))}
            </ul>
          )}
          {open.active && (
            <div className={styles.inlineForm} style={{ marginTop: 12 }}>
              <label className="form-field">Add a role<Select value={newRole} onChange={(e) => setNewRole(e.target.value)} disabled={busy}>{roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}</Select></label>
              <label className="form-field">For<Select value={newScope} onChange={(e) => setNewScope(e.target.value)} disabled={busy}><option value="organization">Whole organization</option>{franchises.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}</Select></label>
              <button type="button" className="primary" disabled={busy || !newRole} onClick={() => void assign(open)}>Add role</button>
            </div>
          )}

          <div style={{ marginTop: 28, paddingTop: 16, borderTop: '1px solid var(--glass-line)' }}>
            {open.active
              ? <button type="button" className="secondary" disabled={busy} onClick={() => setConfirm({ person: open, action: 'terminate' })}>Terminate employee</button>
              : <button type="button" className="primary" disabled={busy} onClick={() => setConfirm({ person: open, action: 'reinstate' })}>Reinstate employee</button>}
            {/* error404 #90: take them off the team for good; their records stay. */}
            <button type="button" className="secondary" disabled={busy} onClick={() => setConfirm({ person: open, action: 'remove' })} style={{ marginLeft: 8 }}>Remove from team</button>
          </div>
        </aside>
      </>}

      <ConfirmDialog isOpen={confirm !== null} title={confirm?.action === 'terminate' ? 'Terminate employee' : confirm?.action === 'remove' ? 'Remove from team' : 'Reinstate employee'} entityName={confirm?.person.fullName || confirm?.person.email || ''} entityType="Employee" actionType={confirm?.action === 'remove' ? 'delete' : 'archive'} isBusy={busy}
        explanation={confirm?.action === 'terminate' ? 'They are signed out everywhere and cannot sign in again. Everything they did stays in your records. You can reinstate them later.' : confirm?.action === 'remove' ? 'They are signed out, cannot sign in, lose all their roles and leave the Team and Employees lists. Bookings, payments and hours they recorded stay in your records under their name. To have them back, invite them again.' : 'They can sign in again with their existing password and roles.'}
        confirmLabel={confirm?.action === 'terminate' ? 'Terminate' : confirm?.action === 'remove' ? 'Remove from team' : 'Reinstate'}
        onConfirm={async () => { if (confirm) await setActive(confirm.person, confirm.action); }} onCancel={() => { if (!busy) setConfirm(null); }} />
    </div>
  );
}
