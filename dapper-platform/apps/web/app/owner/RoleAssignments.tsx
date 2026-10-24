'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import type { Role } from './RoleManager';

type Member = { principalId: string; email: string; active: boolean };
type Named = { id: string; name: string };
type Assignment = { id: string; principalId: string; email: string; roleName: string; scopeType: 'location' | 'franchise' | 'organization'; scopeName: string; version: number };
type Scope = 'organization' | 'franchise' | 'location';

async function readJson(r: Response) {
  if (r.status === 204) return null;
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

export default function RoleAssignments({ organizationId, revision, onSaved }: { organizationId: string; revision: number; onSaved: () => Promise<void> }) {
  const base = `/api/v1/organizations/${organizationId}`;
  const [members, setMembers] = useState<Member[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [franchises, setFranchises] = useState<Named[]>([]);
  const [locations, setLocations] = useState<Named[]>([]);
  const [assignments, setAssignments] = useState<Assignment[] | null>(null);
  const [person, setPerson] = useState('');
  const [roleId, setRoleId] = useState('');
  const [scope, setScope] = useState<Scope>('organization');
  const [target, setTarget] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [revoke, setRevoke] = useState<Assignment | null>(null);

  const load = useCallback(async () => {
    try {
      const [access, structure, r, a] = await Promise.all(['access', 'structure', 'roles', 'role-assignments'].map((p) => fetch(`${base}/${p}`, { cache: 'no-store' }).then(readJson)));
      setMembers((access.members ?? []).filter((m: Member) => m.active));
      setLocations((access.locations ?? []).map((l: Named) => ({ id: l.id, name: l.name })));
      // A franchise role scope uses the franchise grouping id, not the site id.
      setFranchises((structure.franchises ?? []).filter((f: { franchiseId: string | null }) => !!f.franchiseId).map((f: { franchiseId: string; name: string; code: string }) => ({ id: f.franchiseId, name: f.code ? `${f.code} · ${f.name}` : f.name })));
      setRoles(r);
      setAssignments(a);
      setPerson((p) => p || access.members?.find((m: Member) => m.active)?.principalId || '');
      setRoleId((p) => p || r[0]?.id || '');
    } catch (e) {
      setAssignments(null);
      setError(e instanceof Error ? e.message : 'Role assignments could not be loaded.');
    }
  }, [base]);
  useEffect(() => {
    void load();
  }, [load, revision]);

  const targets = scope === 'franchise' ? franchises : scope === 'location' ? locations : [];
  useEffect(() => setTarget(targets[0]?.id ?? ''), [scope, franchises, locations]); // eslint-disable-line react-hooks/exhaustive-deps

  const chosen = roles.find((r) => r.id === roleId);
  // Safer default: only a role that manages roles/access starts as organization-wide; others start scoped to one franchise.
  useEffect(() => {
    const r = roles.find((x) => x.id === roleId);
    if (r) setScope(r.capabilities.includes('roles.manage') || !franchises.length ? 'organization' : 'franchise');
  }, [roleId, roles, franchises.length]);
  const orgOnly = chosen?.capabilities.filter((c) => ['structure.read', 'structure.manage', 'invoices.configure', 'access.manage', 'roles.manage', 'roles.assign'].includes(c)) ?? [];

  async function assign(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await readJson(await fetch(`${base}/role-assignments`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ principalId: person, roleId, scopeType: scope, scopeId: scope === 'organization' ? organizationId : target }) }));
      setNotice(`${chosen?.name ?? 'Role'} assigned.`);
      await load();
      await onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The role could not be assigned.');
    } finally {
      setBusy(false);
    }
  }

  async function doRevoke() {
    if (!revoke) return;
    setBusy(true);
    setError('');
    try {
      await readJson(await fetch(`${base}/role-assignments/${revoke.id}?version=${revoke.version}`, { method: 'DELETE' }));
      setNotice(`${revoke.roleName} removed from ${revoke.email || 'this person'}.`);
      setRevoke(null);
      await load();
      await onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The assignment could not be removed.');
      setRevoke(null);
    } finally {
      setBusy(false);
    }
  }

  const scopeText = (a: Assignment) => (a.scopeType === 'organization' ? 'Whole organization' : `${a.scopeType === 'franchise' ? 'Franchise' : 'Location'}: ${a.scopeName}`);

  return (
    <div className={styles.stack}>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      <form onSubmit={assign} className={styles.resource}>
        <h3>Assign a role</h3>
        {roles.length === 0 && assignments !== null && <p className={styles.personSub}>Create a role first (Roles tab).</p>}
        <div className={styles.inlineForm}>
          <label className="form-field">Person
            <select value={person} onChange={(e) => setPerson(e.target.value)} disabled={busy} required>
              {members.map((m) => <option key={m.principalId} value={m.principalId}>{m.email || 'Account awaiting enrollment'}</option>)}
            </select>
          </label>
          <label className="form-field">Role
            <select value={roleId} onChange={(e) => setRoleId(e.target.value)} disabled={busy} required>
              {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
            </select>
          </label>
          <label className="form-field">Applies to
            <select value={scope} onChange={(e) => setScope(e.target.value as Scope)} disabled={busy}>
              <option value="organization">Whole organization</option>
              <option value="franchise">One franchise</option>
              <option value="location">One location</option>
            </select>
          </label>
          {scope !== 'organization' && (
            <label className="form-field">{scope === 'franchise' ? 'Franchise' : 'Location'}
              <select value={target} onChange={(e) => setTarget(e.target.value)} disabled={busy} required>
                {targets.length === 0 && <option value="">None available</option>}
                {targets.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
              </select>
            </label>
          )}
          <button className="primary" disabled={busy || !person || !roleId || (scope !== 'organization' && !target)}>{busy ? 'Saving…' : 'Assign role'}</button>
        </div>
        {scope !== 'organization' && orgOnly.length > 0 && <p className={styles.footnote}>This role includes organization-wide permissions ({orgOnly.join(', ')}). Those do nothing at {scope} level; only its other permissions apply.</p>}
        {scope === 'organization' && chosen && chosen.capabilities.length > 0 && <p className={styles.footnote}>Organization-wide gives this role everywhere, in every franchise.</p>}
      </form>

      <h3 className={styles.subHead}>Who holds which role</h3>
      {assignments === null && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading assignments" />}
      {assignments?.length === 0 && <p className={styles.status}>No roles assigned yet.</p>}
      <ul className={styles.people}>
        {assignments?.map((a) => (
          <li key={a.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1.2fr) minmax(0,1fr) auto', alignItems: 'center' }}>
            <div className={styles.personMain}><strong className={styles.personName}>{a.email || a.principalId}</strong></div>
            <div className={styles.personWork}><span>{a.roleName}</span><span className={styles.personSub}>{scopeText(a)}</span></div>
            <button type="button" className="secondary" disabled={busy} onClick={() => setRevoke(a)}>Remove</button>
          </li>
        ))}
      </ul>

      <ConfirmDialog isOpen={revoke !== null} title="Remove role" entityName={revoke ? `${revoke.roleName} · ${revoke.email}` : ''} entityType="Role assignment" actionType="delete" isBusy={busy}
        explanation="The person loses everything this role gave them at that scope straight away. The last organization administrator cannot be removed."
        impactNotice="You can assign the role again at any time." confirmLabel="Remove role" onConfirm={doRevoke} onCancel={() => { if (!busy) setRevoke(null); }} />
    </div>
  );
}
