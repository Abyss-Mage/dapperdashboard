'use client';
import { useCallback, useEffect, useState } from 'react';
import MemberInvitations from './MemberInvitations';
import Select from '../GlassSelect';

type Member = {
  principalId: string;
  email: string;
  active: boolean;
};

type Location = {
  id: string;
  name: string;
};

type Franchise = {
  id: string;
  name: string;
};

type Role = {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  version: number;
  capabilities: string[];
  assignmentCount: number;
};

type MemberRoleAssignment = {
  id: string;
  organizationId: string;
  principalId: string;
  email: string;
  roleId: string;
  roleName: string;
  scopeType: 'location' | 'franchise' | 'organization';
  scopeId: string;
  scopeName: string;
  version: number;
  assignedBy: string;
  assignedAt: string;
};

export default function AccessManager({
  organizationId,
  revision,
  onSaved,
  hideInvitations = false,
}: {
  organizationId: string;
  revision: number;
  onSaved: () => Promise<void>;
  hideInvitations?: boolean;
}) {
  const [members, setMembers] = useState<Member[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [franchises, setFranchises] = useState<Franchise[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [assignments, setAssignments] = useState<MemberRoleAssignment[]>([]);

  const [principal, setPrincipal] = useState('');
  const [roleId, setRoleId] = useState('');
  const [scopeType, setScopeType] = useState<'location' | 'franchise'>('location');
  const [scopeId, setScopeId] = useState('');

  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [accessRes, structRes, rolesRes, assignRes] = await Promise.all([
        fetch(`/api/v1/organizations/${organizationId}/access`, { cache: 'no-store' }),
        fetch(`/api/v1/organizations/${organizationId}/structure`, { cache: 'no-store' }),
        fetch(`/api/v1/organizations/${organizationId}/roles`, { cache: 'no-store' }),
        fetch(`/api/v1/organizations/${organizationId}/role-assignments`, { cache: 'no-store' }),
      ]);

      if (!accessRes.ok || !rolesRes.ok || !assignRes.ok) {
        throw new Error('Access settings are unavailable or your authorization has changed.');
      }

      const accessData = await accessRes.json();
      const rolesData: Role[] = await rolesRes.json();
      const assignData: MemberRoleAssignment[] = await assignRes.json();

      let franData: Franchise[] = accessData.franchises || [];
      if (franData.length === 0 && structRes.ok) {
        const s = await structRes.json();
        // A franchise's id is its site id; role assignments scoped to a franchise use the internal grouping id.
        franData = (s.franchises || []).filter((f: { franchiseId?: string | null }) => !!f.franchiseId).map((f: { franchiseId: string; name: string }) => ({ id: f.franchiseId, name: f.name }));
      }

      setMembers(accessData.members || []);
      setLocations(accessData.locations || []);
      setFranchises(franData);
      setRoles(rolesData);
      setAssignments(assignData);

      setPrincipal((prev) =>
        accessData.members?.some((m: Member) => m.principalId === prev)
          ? prev
          : accessData.members?.[0]?.principalId ?? ''
      );
      setRoleId((prev) =>
        rolesData.some((r) => r.id === prev) ? prev : rolesData[0]?.id ?? ''
      );
      setScopeId((prev) => {
        if (scopeType === 'franchise') {
          return franData.some((f) => f.id === prev) ? prev : franData[0]?.id ?? '';
        }
        return accessData.locations?.some((l: Location) => l.id === prev)
          ? prev
          : accessData.locations?.[0]?.id ?? '';
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load access settings.');
    } finally {
      setLoading(false);
    }
  }, [organizationId, scopeType]);

  useEffect(() => {
    loadData().catch((err) => setError(err.message));
  }, [loadData, revision]);

  const selectedMember = members.find((m) => m.principalId === principal);

  async function handleAssign(event: React.FormEvent) {
    event.preventDefault();
    if (!principal || !roleId || !scopeId) return;

    setBusy(true);
    setError('');
    setNotice('');
    try {
      const response = await fetch(`/api/v1/organizations/${organizationId}/role-assignments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          principalId: principal,
          roleId,
          scopeType,
          scopeId,
        }),
      });

      if (!response.ok) {
        const body = await response.json();
        throw new Error(body.error ?? 'Could not assign role.');
      }

      const assignedRole = roles.find((r) => r.id === roleId);
      setNotice(`Role "${assignedRole?.name ?? 'Role'}" assigned successfully.`);
      await loadData();
      await onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not assign role.');
    } finally {
      setBusy(false);
    }
  }

  async function handleRevoke(assignmentId: string, roleName: string, version: number) {
    if (!assignmentId) return;
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const response = await fetch(
        `/api/v1/organizations/${organizationId}/role-assignments/${assignmentId}?version=${version}`,
        { method: 'DELETE' }
      );

      if (!response.ok) {
        const body = await response.json();
        throw new Error(body.error ?? 'Could not revoke role assignment.');
      }

      setNotice(`Assignment for "${roleName}" revoked.`);
      await loadData();
      await onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not revoke assignment.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel detail-card">
      <h2>Member access &amp; role assignment</h2>
      <p>
        Assign an existing named role to an organization member at a selected Location or Franchise scope.
        Role definitions and capability configurations are managed by administrators in the Master Dashboard.
      </p>

      {error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}

      {loading && <p>Loading member access settings…</p>}

      {!loading && roles.length === 0 && (
        <p>
          <em>No named roles are currently defined in this organization. Roles and capability configurations are established in the Master Dashboard.</em>
        </p>
      )}

      {!loading && roles.length > 0 && (
        <form className="management-form" onSubmit={handleAssign}>
          <label className="form-field">
            Member
            <Select
              value={principal}
              onChange={(e) => setPrincipal(e.target.value)}
              disabled={busy}
            >
              {members.map((m) => (
                <option key={m.principalId} value={m.principalId}>
                  {m.email || 'Account awaiting enrollment'}
                  {m.active ? '' : ' (inactive)'}
                </option>
              ))}
            </Select>
          </label>

          <label className="form-field">
            Role
            <Select
              value={roleId}
              onChange={(e) => setRoleId(e.target.value)}
              disabled={busy}
            >
              {roles.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name} ({r.capabilities.length} capabilities)
                </option>
              ))}
            </Select>
          </label>

          <label className="form-field">
            Scope type
            <Select
              value={scopeType}
              onChange={(e) => {
                const next = e.target.value as 'location' | 'franchise';
                setScopeType(next);
                setScopeId(
                  next === 'franchise'
                    ? franchises[0]?.id ?? ''
                    : locations[0]?.id ?? ''
                );
              }}
              disabled={busy}
            >
              <option value="location">Location</option>
              <option value="franchise">Franchise</option>
            </Select>
          </label>

          <label className="form-field">
            Target {scopeType === 'franchise' ? 'franchise' : 'location'}
            <Select
              required
              value={scopeId}
              onChange={(e) => setScopeId(e.target.value)}
              disabled={busy}
            >
              <option value="">
                Choose {scopeType === 'franchise' ? 'franchise' : 'location'}
              </option>
              {scopeType === 'franchise'
                ? franchises.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.name}
                    </option>
                  ))
                : locations.map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name}
                    </option>
                  ))}
            </Select>
          </label>

          <button
            className="primary"
            disabled={
              busy ||
              !principal ||
              !roleId ||
              !scopeId ||
              !selectedMember?.active
            }
          >
            {busy ? 'Saving…' : 'Assign role'}
          </button>
        </form>
      )}

      {!loading && (
        <div style={{ marginTop: '1.5rem' }}>
          <h3>Current role assignments</h3>
          {assignments.length === 0 ? (
            <p>
              <em>No scoped roles currently assigned.</em>
            </p>
          ) : (
            <ul style={{ listStyle: 'none', padding: 0, margin: '0.75rem 0' }}>
              {assignments.map((a) => (
                <li
                  key={a.id}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '0.5rem 0',
                    borderBottom: '1px solid var(--border-subtle, rgba(255,255,255,0.1))',
                    gap: '1rem',
                  }}
                >
                  <div>
                    <strong>{a.email || a.principalId}</strong> &mdash;{' '}
                    <span>{a.roleName}</span>{' '}
                    <small style={{ opacity: 0.8 }}>
                      ({a.scopeType === 'organization'
                        ? 'Organization wide'
                        : `${a.scopeType === 'franchise' ? 'Franchise' : 'Location'}: ${a.scopeName}`})
                    </small>
                  </div>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => handleRevoke(a.id, a.roleName, a.version)}
                    style={{
                      padding: '0.25rem 0.5rem',
                      fontSize: '0.8125rem',
                      cursor: 'pointer',
                    }}
                  >
                    Revoke
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {!loading && !locations.length && (
        <p>Create your actual locations in organization settings before granting operational access.</p>
      )}

      {!hideInvitations && <MemberInvitations key={organizationId} organizationId={organizationId} />}
    </section>
  );
}
