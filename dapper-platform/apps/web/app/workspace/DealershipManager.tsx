'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import Select from '../GlassSelect';
import { ColorPicker } from '../Pickers';

type DealershipTag = {
  id: string;
  dealershipId: string;
  tagName: string;
  tagColor: string;
  createdAt: string;
};

type DealershipAccount = {
  id: string;
  organizationId: string;
  locationId: string;
  customerId: string;
  accountName: string;
  accountCode: string;
  accountType: 'dealership' | 'fleet' | 'commercial';
  contactName: string;
  contactEmail: string;
  contactPhone: string;
  billingTermsDays: number;
  taxExempt: boolean;
  active: boolean;
  notes?: string;
  createdAt: string;
  updatedAt: string;
  tags?: DealershipTag[];
};

type DealershipFleetIntake = {
  id: string;
  organizationId: string;
  locationId: string;
  dealershipId: string;
  vehicleId: string;
  stockNumber: string;
  vin?: string;
  poNumber?: string;
  lotLocation?: string;
  serviceId?: string;
  serviceName: string;
  status: 'intake' | 'queued' | 'in_progress' | 'quality_checked' | 'completed' | 'released';
  jobId?: string;
  assignedPrincipalId?: string;
  intakeNotes?: string;
  receivedAt: string;
  completedAt?: string;
  releasedAt?: string;
  createdAt: string;
  updatedAt: string;
};

type DealershipReport = {
  dealershipId: string;
  accountName: string;
  accountCode: string;
  totalReceived: number;
  totalCompleted: number;
  totalReleased: number;
  activeCount: number;
  invoicedTotalCents: number;
  balanceDueCents: number;
};

type Customer = {
  id: string;
  name: string;
  email: string;
  phone?: string;
};

function formatMoney(cents: number, currency: string = 'CAD'): string {
  if (!Number.isSafeInteger(cents)) return '$0.00';
  return new Intl.NumberFormat('en-CA', { style: 'currency', currency }).format(cents / 100);
}

export default function DealershipManager({
  organizationId,
  locationId,
  canManage,
}: {
  organizationId: string;
  locationId: string;
  canManage: boolean;
}) {
  const [accounts, setAccounts] = useState<DealershipAccount[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [selectedAccountId, setSelectedAccountId] = useState<string | null>(null);

  // Sub-views
  const [activeTab, setActiveTab] = useState<'accounts' | 'queue' | 'report'>('accounts');
  const [intakeItems, setIntakeItems] = useState<DealershipFleetIntake[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [report, setReport] = useState<DealershipReport | null>(null);

  // Account creation form
  const [showCreateAccount, setShowCreateAccount] = useState(false);
  const [customerId, setCustomerId] = useState('');
  const [accountName, setAccountName] = useState('');
  const [accountCode, setAccountCode] = useState('');
  const [accountType, setAccountType] = useState<'dealership' | 'fleet' | 'commercial'>('dealership');
  const [contactName, setContactName] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [contactPhone, setContactPhone] = useState('');
  const [billingTermsDays, setBillingTermsDays] = useState(30);
  const [taxExempt, setTaxExempt] = useState(false);
  const [notes, setNotes] = useState('');

  // Fleet Intake creation form
  const [showCreateIntake, setShowCreateIntake] = useState(false);
  const [vehicleDesc, setVehicleDesc] = useState('');
  const [vehicleCategory, setVehicleCategory] = useState('');
  const [stockNumber, setStockNumber] = useState('');
  const [vin, setVin] = useState('');
  const [poNumber, setPoNumber] = useState('');
  const [lotLocation, setLotLocation] = useState('');
  const [serviceName, setServiceName] = useState('Standard Fleet Detail');
  const [intakeNotes, setIntakeNotes] = useState('');

  // Tag creation
  const [showAddTag, setShowAddTag] = useState<string | null>(null);
  const [newTagName, setNewTagName] = useState('');
  const [newTagColor, setNewTagColor] = useState('#d10f1f');

  const errorRef = useRef<HTMLParagraphElement>(null);
  const base = `/api/v1/organizations/${organizationId}/locations/${locationId}/dealerships`;

  const loadAccounts = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fetch(base, { cache: 'no-store' });
      if (!res.ok) throw new Error('Could not load dealership accounts.');
      const data = await res.json();
      setAccounts(data.items || []);
      if (!selectedAccountId && data.items?.length > 0) {
        setSelectedAccountId(data.items[0].id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Dealership accounts unavailable.');
    } finally {
      setLoading(false);
    }
  }, [base, selectedAccountId]);

  const loadCustomers = useCallback(async () => {
    try {
      const res = await fetch(`/api/v1/organizations/${organizationId}/locations/${locationId}/customers`, { cache: 'no-store' });
      if (res.ok) {
        const data = await res.json();
        setCustomers(data.items || []);
        if (data.items?.length > 0 && !customerId) {
          setCustomerId(data.items[0].id);
        }
      }
    } catch {
      // non-critical if customers list fails to load
    }
  }, [organizationId, locationId, customerId]);

  const loadIntake = useCallback(async (dealerId: string, filter: string = '') => {
    setError('');
    try {
      const url = `${base}/${dealerId}/intake${filter ? `?status=${encodeURIComponent(filter)}` : ''}`;
      const res = await fetch(url, { cache: 'no-store' });
      if (!res.ok) throw new Error('Could not load fleet intake queue.');
      const data = await res.json();
      setIntakeItems(data.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Fleet intake unavailable.');
    }
  }, [base]);

  const loadReport = useCallback(async (dealerId: string) => {
    setError('');
    try {
      const res = await fetch(`${base}/${dealerId}/report`, { cache: 'no-store' });
      if (!res.ok) throw new Error('Could not load dealership report.');
      const data = await res.json();
      setReport(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Report unavailable.');
    }
  }, [base]);

  useEffect(() => {
    loadAccounts();
    loadCustomers();
  }, [loadAccounts, loadCustomers]);

  useEffect(() => {
    if (selectedAccountId) {
      if (activeTab === 'queue') {
        loadIntake(selectedAccountId, statusFilter);
      } else if (activeTab === 'report') {
        loadReport(selectedAccountId);
      }
    }
  }, [selectedAccountId, activeTab, statusFilter, loadIntake, loadReport]);

  useEffect(() => {
    if (error && !loading) errorRef.current?.focus();
  }, [error, loading]);

  async function handleCreateAccount(e: React.FormEvent) {
    e.preventDefault();
    if (!canManage) return;
    setError('');
    setNotice('');

    try {
      const res = await fetch(base, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          customerId,
          accountName,
          accountCode: accountCode.toUpperCase().trim(),
          accountType,
          contactName,
          contactEmail,
          contactPhone,
          billingTermsDays: Number(billingTermsDays),
          taxExempt,
          notes,
        }),
      });

      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to create dealership account');
      }

      const created: DealershipAccount = await res.json();
      setNotice(`Dealership account "${created.accountName}" (${created.accountCode}) created!`);
      setShowCreateAccount(false);
      setAccountName('');
      setAccountCode('');
      setContactName('');
      setContactEmail('');
      setContactPhone('');
      setNotes('');
      setSelectedAccountId(created.id);
      loadAccounts();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create account');
    }
  }

  async function handleCreateIntake(e: React.FormEvent) {
    e.preventDefault();
    if (!canManage || !selectedAccountId) return;
    setError('');
    setNotice('');

    try {
      const res = await fetch(`${base}/${selectedAccountId}/intake`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          vehicleDescription: vehicleDesc,
          categoryDescription: vehicleCategory,
          stockNumber,
          vin,
          poNumber,
          lotLocation,
          serviceName,
          intakeNotes,
        }),
      });

      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to record fleet vehicle intake');
      }

      const created: DealershipFleetIntake = await res.json();
      setNotice(`Vehicle Stock #${created.stockNumber} received into fleet queue!`);
      setShowCreateIntake(false);
      setVehicleDesc('');
      setVehicleCategory('');
      setStockNumber('');
      setVin('');
      setPoNumber('');
      setLotLocation('');
      setIntakeNotes('');
      loadIntake(selectedAccountId, statusFilter);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not record intake');
    }
  }

  async function handleUpdateStatus(intakeId: string, newStatus: string) {
    if (!canManage || !selectedAccountId) return;
    setError('');
    setNotice('');

    try {
      const res = await fetch(`${base}/${selectedAccountId}/intake/${intakeId}/status`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          status: newStatus,
          notes: `Status updated to ${newStatus}`,
        }),
      });

      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to update vehicle status');
      }

      setNotice(`Vehicle status updated to ${newStatus.toUpperCase()}`);
      loadIntake(selectedAccountId, statusFilter);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update status');
    }
  }

  async function handleAddTag(dealerId: string) {
    if (!canManage || !newTagName.trim()) return;
    setError('');
    setNotice('');

    try {
      const res = await fetch(`${base}/${dealerId}/tags`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          tagName: newTagName.trim(),
          tagColor: newTagColor,
        }),
      });

      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to add tag');
      }

      setNotice(`Tag "${newTagName}" added!`);
      setShowAddTag(null);
      setNewTagName('');
      loadAccounts();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not add tag');
    }
  }

  async function handleRemoveTag(dealerId: string, tagId: string) {
    if (!canManage) return;
    setError('');
    setNotice('');

    try {
      const res = await fetch(`${base}/${dealerId}/tags/${tagId}`, {
        method: 'DELETE',
      });

      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to remove tag');
      }

      setNotice('Tag removed.');
      loadAccounts();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not remove tag');
    }
  }

  const selectedAccount = accounts.find((a) => a.id === selectedAccountId);

  return (
    <section className="panel detail-card" style={{ marginTop: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div>
          <h3>Dealership & B2B Fleet Management</h3>
          <p style={{ margin: '0.2rem 0', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            Corporate fleet accounts, vehicle stock intake queue, service progression, and aggregated billing.
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button type="button" className="secondary" onClick={loadAccounts} disabled={loading}>
            Refresh
          </button>
          {canManage && (
            <button
              type="button"
              className="primary"
              onClick={() => setShowCreateAccount(!showCreateAccount)}
            >
              {showCreateAccount ? 'Cancel' : '+ New Corporate Account'}
            </button>
          )}
        </div>
      </div>

      {notice && <p role="status" style={{ color: 'var(--success)', fontWeight: 600, margin: '0.5rem 0' }}>{notice}</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1} className="field-error" style={{ margin: '0.5rem 0' }}>{error}</p>}

      {/* Account Creation Modal / Form */}
      {showCreateAccount && (
        <form onSubmit={handleCreateAccount} style={{ background: 'var(--glass-strong)', padding: '1rem', borderRadius: '4px', margin: '1rem 0' }}>
          <h4>Create Corporate / Dealership Account</h4>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '0.75rem', marginTop: '0.5rem' }}>
            <label className="form-field">
              Billing Customer
              <Select value={customerId} onChange={(e) => setCustomerId(e.target.value)} required>
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>{c.name} ({c.email})</option>
                ))}
              </Select>
            </label>
            <label className="form-field">
              Account Name
              <input
                type="text"
                placeholder="e.g. BMW Midtown Oakville"
                value={accountName}
                onChange={(e) => setAccountName(e.target.value)}
                required
                minLength={2}
                maxLength={160}
              />
            </label>
            <label className="form-field">
              Account Code
              <input
                type="text"
                placeholder="e.g. BMW-MIDTOWN"
                value={accountCode}
                onChange={(e) => setAccountCode(e.target.value)}
                required
                minLength={2}
                maxLength={32}
              />
            </label>
            <label className="form-field">
              Account Type
              <Select value={accountType} onChange={(e) => setAccountType(e.target.value as any)}>
                <option value="dealership">Dealership</option>
                <option value="fleet">Commercial Fleet</option>
                <option value="commercial">Commercial General</option>
              </Select>
            </label>
            <label className="form-field">
              Primary Contact Name
              <input
                type="text"
                placeholder="e.g. Sarah Connor"
                value={contactName}
                onChange={(e) => setContactName(e.target.value)}
                required
                minLength={2}
                maxLength={120}
              />
            </label>
            <label className="form-field">
              Contact Email
              <input
                type="email"
                placeholder="fleet@dealer.com"
                value={contactEmail}
                onChange={(e) => setContactEmail(e.target.value)}
                required
              />
            </label>
            <label className="form-field">
              Contact Phone
              <input
                type="tel"
                placeholder="416-555-0199"
                value={contactPhone}
                onChange={(e) => setContactPhone(e.target.value)}
              />
            </label>
            <label className="form-field">
              Payment Terms (Days)
              <input
                type="number"
                min={0}
                max={180}
                value={billingTermsDays}
                onChange={(e) => setBillingTermsDays(Number(e.target.value))}
                required
              />
            </label>
          </div>
          <div style={{ margin: '0.75rem 0' }}>
            <label style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={taxExempt}
                onChange={(e) => setTaxExempt(e.target.checked)}
              />
              <span>Tax Exempt (Commercial Wholesale Agreement)</span>
            </label>
          </div>
          <label className="form-field">
            Account Notes
            <textarea
              placeholder="Internal fleet preferences, key drop-off instructions, etc."
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              maxLength={2000}
            />
          </label>
          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem' }}>
            <button type="submit" className="primary">Save Account</button>
            <button type="button" className="secondary" onClick={() => setShowCreateAccount(false)}>Cancel</button>
          </div>
        </form>
      )}

      {/* Account Selector & Navigation Tabs */}
      {accounts.length === 0 ? (
        <p style={{ marginTop: '1rem' }}>No dealership or commercial fleet accounts registered yet.</p>
      ) : (
        <div style={{ marginTop: '1rem' }}>
          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap', marginBottom: '1rem' }}>
            <label style={{ fontWeight: 600, fontSize: '0.9rem' }}>Select Account:</label>
            <Select
              value={selectedAccountId || ''}
              onChange={(e) => setSelectedAccountId(e.target.value)}
              style={{ padding: '0.4rem', minWidth: '240px' }}
            >
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.accountName} ({a.accountCode}) — {a.accountType.toUpperCase()}
                </option>
              ))}
            </Select>

            <div style={{ display: 'flex', gap: '0.25rem', marginLeft: 'auto' }}>
              <button
                type="button"
                className={activeTab === 'accounts' ? 'primary' : 'secondary'}
                onClick={() => setActiveTab('accounts')}
                style={{ fontSize: '0.85rem', padding: '0.35rem 0.65rem' }}
              >
                Account Details
              </button>
              <button
                type="button"
                className={activeTab === 'queue' ? 'primary' : 'secondary'}
                onClick={() => setActiveTab('queue')}
                style={{ fontSize: '0.85rem', padding: '0.35rem 0.65rem' }}
              >
                Fleet Queue
              </button>
              <button
                type="button"
                className={activeTab === 'report' ? 'primary' : 'secondary'}
                onClick={() => setActiveTab('report')}
                style={{ fontSize: '0.85rem', padding: '0.35rem 0.65rem' }}
              >
                Period Report
              </button>
            </div>
          </div>

          {/* TAB 1: Account Details */}
          {activeTab === 'accounts' && selectedAccount && (
            <div style={{ border: '1px solid var(--glass-line)', borderRadius: '4px', padding: '1rem', background: 'var(--glass-strong)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap' }}>
                <h4 style={{ margin: 0 }}>{selectedAccount.accountName} ({selectedAccount.accountCode})</h4>
                <span
                  style={{
                    padding: '0.2rem 0.6rem',
                    borderRadius: '3px',
                    fontSize: '0.8rem',
                    fontWeight: 700,
                    backgroundColor: selectedAccount.active ? 'color-mix(in srgb, var(--success) 16%, transparent)' : 'color-mix(in srgb, var(--error) 16%, transparent)',
                    color: selectedAccount.active ? 'var(--success)' : 'var(--error)',
                  }}
                >
                  {selectedAccount.active ? 'ACTIVE' : 'INACTIVE'}
                </span>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0.75rem', margin: '0.75rem 0', fontSize: '0.9rem' }}>
                <div><strong>Type:</strong> {selectedAccount.accountType.toUpperCase()}</div>
                <div><strong>Contact:</strong> {selectedAccount.contactName}</div>
                <div><strong>Email:</strong> {selectedAccount.contactEmail}</div>
                <div><strong>Phone:</strong> {selectedAccount.contactPhone || 'N/A'}</div>
                <div><strong>Terms:</strong> Net {selectedAccount.billingTermsDays} Days</div>
                <div><strong>Tax Status:</strong> {selectedAccount.taxExempt ? 'Wholesale Exempt' : 'Taxable (HST)'}</div>
              </div>

              {selectedAccount.notes && (
                <p style={{ background: 'var(--glass-strong)', padding: '0.5rem', borderRadius: '4px', fontStyle: 'italic', fontSize: '0.85rem' }}>
                  {selectedAccount.notes}
                </p>
              )}

              {/* Tags Section */}
              <div style={{ marginTop: '1rem', borderTop: '1px solid var(--glass-line)', paddingTop: '0.75rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
                  <strong style={{ fontSize: '0.85rem' }}>Account Tags:</strong>
                  {selectedAccount.tags && selectedAccount.tags.length > 0 ? (
                    selectedAccount.tags.map((t) => (
                      <span
                        key={t.id}
                        style={{
                          backgroundColor: t.tagColor,
                          color: '#fff',
                          padding: '0.15rem 0.5rem',
                          borderRadius: '12px',
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '0.3rem',
                        }}
                      >
                        {t.tagName}
                        {canManage && (
                          <button
                            type="button"
                            onClick={() => handleRemoveTag(selectedAccount.id, t.id)}
                            style={{ background: 'transparent', border: 0, color: '#fff', cursor: 'pointer', padding: 0, fontSize: '0.8rem', lineHeight: 1 }}
                            title="Remove tag"
                          >
                            ×
                          </button>
                        )}
                      </span>
                    ))
                  ) : (
                    <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>No tags added</span>
                  )}
                  {canManage && (
                    <button
                      type="button"
                      className="secondary"
                      onClick={() => setShowAddTag(selectedAccount.id)}
                      style={{ fontSize: '0.75rem', padding: '0.2rem 0.5rem' }}
                    >
                      + Add Tag
                    </button>
                  )}
                </div>

                {showAddTag === selectedAccount.id && (
                  <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginTop: '0.5rem', flexWrap: 'wrap' }}>
                    <input
                      type="text"
                      placeholder="Tag Name (e.g. VIP-LOT)"
                      value={newTagName}
                      onChange={(e) => setNewTagName(e.target.value)}
                      style={{ padding: '0.3rem', fontSize: '0.8rem' }}
                    />
                    <ColorPicker
                      value={newTagColor}
                      onChange={(e) => setNewTagColor(e.target.value)}
                      style={{ width: '32px', height: '28px', padding: 0, border: 0 }}
                      title="Tag Color"
                    />
                    <button
                      type="button"
                      className="primary"
                      onClick={() => handleAddTag(selectedAccount.id)}
                      style={{ fontSize: '0.8rem', padding: '0.3rem 0.6rem' }}
                    >
                      Save
                    </button>
                    <button
                      type="button"
                      className="secondary"
                      onClick={() => setShowAddTag(null)}
                      style={{ fontSize: '0.8rem', padding: '0.3rem 0.6rem' }}
                    >
                      Cancel
                    </button>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* TAB 2: Fleet Intake Queue */}
          {activeTab === 'queue' && selectedAccount && (
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '0.75rem' }}>
                <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                  <label style={{ fontSize: '0.85rem', fontWeight: 600 }}>Filter by Status:</label>
                  <Select
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                    style={{ padding: '0.3rem', fontSize: '0.85rem' }}
                  >
                    <option value="">All Statuses</option>
                    <option value="intake">Intake (Received)</option>
                    <option value="queued">Queued</option>
                    <option value="in_progress">In Progress</option>
                    <option value="quality_checked">Quality Checked</option>
                    <option value="completed">Completed</option>
                    <option value="released">Released to Dealer</option>
                  </Select>
                </div>
                {canManage && (
                  <button
                    type="button"
                    className="primary"
                    onClick={() => setShowCreateIntake(!showCreateIntake)}
                    style={{ fontSize: '0.85rem', padding: '0.35rem 0.75rem' }}
                  >
                    {showCreateIntake ? 'Cancel' : '+ Intake Vehicle'}
                  </button>
                )}
              </div>

              {/* Intake Form */}
              {showCreateIntake && (
                <form onSubmit={handleCreateIntake} style={{ background: 'var(--glass-strong)', padding: '1rem', borderRadius: '4px', marginBottom: '1rem' }}>
                  <h5>Record Fleet Vehicle Intake ({selectedAccount.accountName})</h5>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0.5rem', marginTop: '0.5rem' }}>
                    <label className="form-field">
                      Vehicle Description
                      <input
                        type="text"
                        placeholder="e.g. 2026 BMW M3 Sedan"
                        value={vehicleDesc}
                        onChange={(e) => setVehicleDesc(e.target.value)}
                        required
                      />
                    </label>
                    <label className="form-field">
                      Stock Number
                      <input
                        type="text"
                        placeholder="e.g. STK-9901-M3"
                        value={stockNumber}
                        onChange={(e) => setStockNumber(e.target.value)}
                        required
                      />
                    </label>
                    <label className="form-field">
                      VIN (optional)
                      <input
                        type="text"
                        placeholder="17-character VIN"
                        value={vin}
                        onChange={(e) => setVin(e.target.value)}
                        maxLength={30}
                      />
                    </label>
                    <label className="form-field">
                      PO / Tag Number
                      <input
                        type="text"
                        placeholder="e.g. PO-2026-4412"
                        value={poNumber}
                        onChange={(e) => setPoNumber(e.target.value)}
                      />
                    </label>
                    <label className="form-field">
                      Lot / Parking Location
                      <input
                        type="text"
                        placeholder="e.g. Bay 3 Rear"
                        value={lotLocation}
                        onChange={(e) => setLotLocation(e.target.value)}
                      />
                    </label>
                    <label className="form-field">
                      Requested Service
                      <input
                        type="text"
                        value={serviceName}
                        onChange={(e) => setServiceName(e.target.value)}
                        required
                      />
                    </label>
                  </div>
                  <label className="form-field" style={{ marginTop: '0.5rem' }}>
                    Intake Notes
                    <input
                      type="text"
                      placeholder="Rush order, delivery by 4pm, specific spot cleaning, etc."
                      value={intakeNotes}
                      onChange={(e) => setIntakeNotes(e.target.value)}
                    />
                  </label>
                  <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.5rem' }}>
                    <button type="submit" className="primary">Add to Fleet Queue</button>
                    <button type="button" className="secondary" onClick={() => setShowCreateIntake(false)}>Cancel</button>
                  </div>
                </form>
              )}

              {intakeItems.length === 0 ? (
                <p>No vehicles found in fleet intake queue matching filter.</p>
              ) : (
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', fontSize: '0.85rem', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr style={{ borderBottom: '2px solid var(--glass-line)', textAlign: 'left' }}>
                        <th style={{ padding: '0.4rem' }}>Stock #</th>
                        <th style={{ padding: '0.4rem' }}>Vehicle</th>
                        <th style={{ padding: '0.4rem' }}>PO / Tag</th>
                        <th style={{ padding: '0.4rem' }}>Service</th>
                        <th style={{ padding: '0.4rem' }}>Status</th>
                        <th style={{ padding: '0.4rem' }}>Received</th>
                        {canManage && <th style={{ padding: '0.4rem' }}>Actions</th>}
                      </tr>
                    </thead>
                    <tbody>
                      {intakeItems.map((item) => (
                        <tr key={item.id} style={{ borderBottom: '1px solid var(--glass-line)' }}>
                          <td style={{ padding: '0.4rem', fontWeight: 700 }}>{item.stockNumber}</td>
                          <td style={{ padding: '0.4rem' }}>
                            {item.vin ? <code style={{ fontSize: '0.75rem' }}>{item.vin}</code> : 'VIN N/A'}
                            {item.lotLocation && <span style={{ color: 'var(--text-muted)' }}> · {item.lotLocation}</span>}
                          </td>
                          <td style={{ padding: '0.4rem' }}>{item.poNumber || '—'}</td>
                          <td style={{ padding: '0.4rem' }}>{item.serviceName}</td>
                          <td style={{ padding: '0.4rem' }}>
                            <span
                              style={{
                                padding: '0.15rem 0.45rem',
                                borderRadius: '3px',
                                fontSize: '0.75rem',
                                fontWeight: 700,
                                backgroundColor:
                                  item.status === 'completed' || item.status === 'released'
                                    ? 'color-mix(in srgb, var(--success) 16%, transparent)'
                                    : item.status === 'in_progress'
                                    ? 'var(--gold-tint)'
                                    : 'var(--neutral-chip)',
                                color:
                                  item.status === 'completed' || item.status === 'released'
                                    ? 'var(--success)'
                                    : item.status === 'in_progress'
                                    ? 'var(--on-gold-tint)'
                                    : '#d10f1f',
                              }}
                            >
                              {item.status.toUpperCase().replace('_', ' ')}
                            </span>
                          </td>
                          <td style={{ padding: '0.4rem', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                            {new Date(item.receivedAt).toLocaleDateString()}
                          </td>
                          {canManage && (
                            <td style={{ padding: '0.4rem' }}>
                              <Select
                                value={item.status}
                                onChange={(e) => handleUpdateStatus(item.id, e.target.value)}
                                style={{ fontSize: '0.75rem', padding: '0.2rem' }}
                              >
                                <option value="intake">Intake</option>
                                <option value="queued">Queued</option>
                                <option value="in_progress">In Progress</option>
                                <option value="quality_checked">QC Inspection</option>
                                <option value="completed">Completed</option>
                                <option value="released">Release to Dealer</option>
                              </Select>
                            </td>
                          )}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {/* TAB 3: Period Report */}
          {activeTab === 'report' && selectedAccount && (
            <div>
              {report ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '0.75rem' }}>
                    <div style={{ background: 'var(--glass-strong)', padding: '0.75rem', borderRadius: '4px', textAlign: 'center' }}>
                      <div style={{ fontSize: '1.4rem', fontWeight: 700 }}>{report.totalReceived}</div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Total Received</div>
                    </div>
                    <div style={{ background: 'var(--neutral-chip)', padding: '0.75rem', borderRadius: '4px', textAlign: 'center' }}>
                      <div style={{ fontSize: '1.4rem', fontWeight: 700, color: 'var(--link)' }}>{report.activeCount}</div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--link)' }}>In Progress / Queued</div>
                    </div>
                    <div style={{ background: 'color-mix(in srgb, var(--success) 16%, transparent)', padding: '0.75rem', borderRadius: '4px', textAlign: 'center' }}>
                      <div style={{ fontSize: '1.4rem', fontWeight: 700, color: 'var(--success)' }}>{report.totalCompleted}</div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--success)' }}>Completed Details</div>
                    </div>
                    <div style={{ background: 'var(--neutral-chip)', padding: '0.75rem', borderRadius: '4px', textAlign: 'center' }}>
                      <div style={{ fontSize: '1.4rem', fontWeight: 700, color: 'var(--text)' }}>{report.totalReleased}</div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--text)' }}>Released to Dealer</div>
                    </div>
                  </div>

                  <div style={{ background: 'var(--glass-strong)', padding: '1rem', borderRadius: '4px' }}>
                    <h5 style={{ margin: '0 0 0.5rem' }}>Billing & Ledger Balance</h5>
                    <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', fontSize: '0.95rem' }}>
                      <div>
                        <strong>Invoiced Total:</strong> {formatMoney(report.invoicedTotalCents)}
                      </div>
                      <div>
                        <strong>Outstanding Balance:</strong>{' '}
                        <span style={{ color: report.balanceDueCents > 0 ? 'var(--error)' : 'var(--success)', fontWeight: 700 }}>
                          {formatMoney(report.balanceDueCents)}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                <p>Loading dealership period report…</p>
              )}
            </div>
          )}
        </div>
      )}
    </section>
  );
}
