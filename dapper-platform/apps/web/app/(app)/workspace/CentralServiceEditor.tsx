'use client';
import { useCallback, useEffect, useState } from 'react';
import ConfirmDialog from './ConfirmDialog';
import Select from '../GlassSelect';

type CentralService = {
  id: string;
  name: string;
  kind: string;
  parentId: string | null;
  version: number;
  archived: boolean;
};

type Org = {
  id: string;
  name: string;
  actions: string[];
};

export default function CentralServiceEditor({ organization, canManage }: { organization: Org; canManage: boolean }) {
  const [items, setItems] = useState<CentralService[]>([]);
  const [editing, setEditing] = useState<CentralService | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<{
    action: 'delete' | 'archive';
    item: CentralService;
  } | null>(null);
  const [name, setName] = useState('');
  const [kind, setKind] = useState('main');
  const [parentId, setParentId] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const base = `/api/v1/organizations/${organization.id}/central-services`;

  const load = useCallback(async () => {
    const r = await fetch(base);
    if (!r.ok) throw new Error('Could not load organization central services');
    const data = await r.json();
    setItems(data.items);
  }, [base]);

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, [load]);

  function clear() {
    setEditing(null);
    setName('');
    setKind('main');
    setParentId('');
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(base + (editing ? '/' + editing.id : ''), {
        method: editing ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          kind,
          parentId: kind === 'addon' ? parentId : null,
          ...(editing ? { version: editing.version } : {}),
        }),
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'Could not save central service');
        return;
      }
      setNotice(`Central service "${name}" saved.`);
      clear();
      await load();
    } catch {
      setError('Could not reach the workspace. Input is preserved.');
    } finally {
      setBusy(false);
    }
  }

  async function archive(item: CentralService) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(`${base}/${item.id}/archive`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version: item.version }),
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not archive central service');
        return;
      }
      setNotice(`Central service "${item.name}" archived.`);
      if (editing?.id === item.id) clear();
      await load();
    } catch {
      setError('Could not archive central service. Refresh before retrying.');
    } finally {
      setBusy(false);
    }
  }

  async function deleteCentralService(item: CentralService) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(`${base}/${item.id}?version=${item.version}`, {
        method: 'DELETE',
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not delete central service');
        return;
      }
      const data = await r.json();
      setNotice(data.message ?? `Central service "${item.name}" removed.`);
      if (editing?.id === item.id) clear();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not delete central service.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel detail-card">
      <h2>Central catalogue · {organization.name}</h2>
      <p><a href="/content">Manage service publication &amp; portfolio images ↗</a></p>
      <p>
        Organization-level services and add-ons. Available across all authorized locations in this organization.
        Revision snapshots are preserved; archiving retains full job and audit history. Deleting unused services removes them permanently; deleting referenced services removes them from the active catalogue while retaining historical records.
      </p>
      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      {canManage && (
        <form className="management-form" onSubmit={save}>
          <label className="form-field">
            Central service name
            <input required minLength={2} maxLength={120} value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <label className="form-field">
            Type
            <Select
              value={kind}
              onChange={(e) => {
                setKind(e.target.value);
                setParentId('');
              }}
            >
              <option value="main">Main service</option>
              <option value="addon">Add-on</option>
            </Select>
          </label>
          {kind === 'addon' && (
            <label className="form-field">
              Parent central service
              <Select required value={parentId} onChange={(e) => setParentId(e.target.value)}>
                <option value="">Choose parent main service</option>
                {items
                  .filter((s) => !s.archived && s.kind === 'main' && s.id !== editing?.id)
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
              </Select>
            </label>
          )}
          <button className="primary" disabled={busy}>
            {editing ? 'Save central revision' : 'Add central service'}
          </button>
          {editing && (
            <button type="button" className="secondary" onClick={clear}>
              Cancel edit
            </button>
          )}
        </form>
      )}
      <div className="management-list">
        {items.map((item) => (
          <article key={item.id}>
            <div>
              <strong>{item.name}</strong>
              <small>
                {item.kind === 'addon'
                  ? `Add-on to ${items.find((s) => s.id === item.parentId)?.name ?? 'main service'}`
                  : 'Main service'}{' '}
                · revision {item.version}
                {item.archived ? ' · Archived' : ''}
              </small>
            </div>
            {canManage && !item.archived && (
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => {
                    setEditing(item);
                    setName(item.name);
                    setKind(item.kind);
                    setParentId(item.parentId ?? '');
                  }}
                >
                  Edit
                </button>
                <button className="secondary" disabled={busy} onClick={() => setConfirmTarget({ action: 'archive', item })}>
                  Archive
                </button>
                <button className="secondary" disabled={busy} onClick={() => setConfirmTarget({ action: 'delete', item })}>
                  Delete
                </button>
              </div>
            )}
          </article>
        ))}
        {!items.length && <p>No central services in this organization catalogue.</p>}
      </div>

      <ConfirmDialog
        isOpen={confirmTarget !== null}
        title={confirmTarget?.action === 'delete' ? 'Delete Central Service' : 'Archive Central Service'}
        entityName={confirmTarget?.item.name ?? ''}
        entityType="Central Service"
        actionType={confirmTarget?.action ?? 'delete'}
        explanation={
          confirmTarget?.action === 'delete'
            ? 'Deleting an unused central service removes it permanently. If it has historical records across organization locations, it will be unlisted while preserving all past jobs, bookings, and invoices.'
            : 'Archiving removes this central service from active location catalogues and scheduling while retaining complete historical and audit records.'
        }
        impactNotice={
          confirmTarget?.item.kind === 'main'
            ? 'Locations with linked services referencing this parent service will have those items unlinked from future imports.'
            : undefined
        }
        isBusy={busy}
        onConfirm={async () => {
          if (!confirmTarget) return;
          const target = confirmTarget;
          setConfirmTarget(null);
          if (target.action === 'delete') {
            await deleteCentralService(target.item);
          } else {
            await archive(target.item);
          }
        }}
        onCancel={() => setConfirmTarget(null)}
      />
    </section>
  );
}
