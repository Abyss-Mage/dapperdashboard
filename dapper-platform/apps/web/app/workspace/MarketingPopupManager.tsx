'use client';

import { useCallback, useEffect, useState } from 'react';

type PopupFormField = {
  name: string;
  type: string;
  label: string;
  required: boolean;
};

type PopupForm = {
  id: string;
  organizationId: string;
  locationId: string;
  name: string;
  title: string;
  description: string;
  headline: string;
  fields: PopupFormField[];
  audienceId: string;
  tags: string[];
  displayDelaySeconds: number;
  selectedPages: string[];
  repeatDisplayLimitDays: number;
  successMessage: string;
  active: boolean;
  createdAt: string;
  updatedAt: string;
};

type PopupSubmission = {
  id: string;
  formId: string;
  organizationId: string;
  locationId: string;
  email: string;
  firstName: string;
  lastName: string;
  metadata: Record<string, unknown>;
  ipAddress: string;
  userAgent: string;
  submittedAt: string;
};

type MarketingConsent = {
  id: string;
  organizationId: string;
  locationId: string;
  customerId: string | null;
  email: string;
  firstName: string;
  lastName: string;
  consentStatus: string;
  consentSource: string;
  consentText: string;
  ipAddress: string;
  userAgent: string;
  consentedAt: string;
  revokedAt: string | null;
  syncStatus: string;
  mailchimpSubscriberHash: string;
  lastSyncedAt: string | null;
  lastSyncError: string;
};

type MailchimpSettings = {
  id: string;
  organizationId: string;
  locationId: string;
  apiKeyConfigured: boolean;
  serverPrefix: string;
  defaultListId: string;
  doubleOptIn: boolean;
  syncEnabled: boolean;
};

export default function MarketingPopupManager({
  organizationId,
  locationId,
  canManage,
}: {
  organizationId: string;
  locationId: string;
  canManage: boolean;
}) {
  const [tab, setTab] = useState<'popups' | 'consents' | 'settings'>('settings');
  const [popups, setPopups] = useState<PopupForm[]>([]);
  const [consents, setConsents] = useState<MarketingConsent[]>([]);
  const [settings, setSettings] = useState<MailchimpSettings | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Selected form submissions view
  const [inspectForm, setInspectForm] = useState<PopupForm | null>(null);
  const [submissions, setSubmissions] = useState<PopupSubmission[]>([]);

  // Create form modal state
  const [creating, setCreating] = useState(false);
  const [formName, setFormName] = useState('');
  const [formTitle, setFormTitle] = useState('');
  const [formHeadline, setFormHeadline] = useState('');
  const [formDescription, setFormDescription] = useState('');
  const [formSuccessMessage, setFormSuccessMessage] = useState('Thank you for subscribing!');
  const [formDelay, setFormDelay] = useState(5);
  const [formCooldown, setFormCooldown] = useState(7);
  const [formAudienceId, setFormAudienceId] = useState('');
  const [formTags, setFormTags] = useState('WebsiteLead, Promo');
  const [formPages, setFormPages] = useState('/');

  // Settings form state
  const [serverPrefix, setServerPrefix] = useState('');
  const [defaultListId, setDefaultListId] = useState('');
  const [doubleOptIn, setDoubleOptIn] = useState(false);
  const [syncEnabled, setSyncEnabled] = useState(true);

  const base = `/api/v1/organizations/${organizationId}/locations/${locationId}/marketing`;

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [popupsRes, consentsRes, settingsRes] = await Promise.all([
        fetch(`${base}/popups`),
        fetch(`${base}/consents?limit=50`),
        fetch(`${base}/settings`),
      ]);

      if (popupsRes.ok) {
        const data = await popupsRes.json();
        setPopups(data || []);
      }
      if (consentsRes.ok) {
        const data = await consentsRes.json();
        setConsents(data.consents || []);
      }
      if (settingsRes.ok) {
        const data = await settingsRes.json();
        setSettings(data);
        setServerPrefix(data.serverPrefix || '');
        setDefaultListId(data.defaultListId || '');
        setDoubleOptIn(data.doubleOptIn || false);
        setSyncEnabled(data.syncEnabled ?? true);
      }
    } catch {
      setError('Failed to load marketing information.');
    } finally {
      setLoading(false);
    }
  }, [base]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleCreateForm = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canManage) return;
    setError('');
    setSuccess('');
    try {
      const pages = formPages.split(',').map((p) => p.trim()).filter(Boolean);
      const tags = formTags.split(',').map((t) => t.trim()).filter(Boolean);
      const res = await fetch(`${base}/popups`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: formName,
          title: formTitle,
          headline: formHeadline,
          description: formDescription,
          successMessage: formSuccessMessage,
          displayDelaySeconds: formDelay,
          repeatDisplayLimitDays: formCooldown,
          audienceId: formAudienceId,
          tags,
          selectedPages: pages.length > 0 ? pages : ['/'],
          fields: [
            { name: 'email', type: 'email', label: 'Email Address', required: true },
            { name: 'first_name', type: 'text', label: 'First Name', required: false },
          ],
          active: true,
        }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to create popup form.');
      }
      setSuccess('Popup form created successfully.');
      setCreating(false);
      setFormName('');
      setFormTitle('');
      setFormHeadline('');
      setFormDescription('');
      loadData();
    } catch (err: unknown) {
      if (err instanceof Error) setError(err.message);
      else setError('Error creating form.');
    }
  };

  const handleToggleActive = async (form: PopupForm) => {
    if (!canManage) return;
    try {
      const res = await fetch(`${base}/popups/${form.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...form,
          active: !form.active,
        }),
      });
      if (res.ok) {
        loadData();
      }
    } catch {
      setError('Failed to update form status.');
    }
  };

  const handleDeleteForm = async (id: string) => {
    if (!canManage || !confirm('Are you sure you want to delete this popup form?')) return;
    try {
      const res = await fetch(`${base}/popups/${id}`, { method: 'DELETE' });
      if (res.ok) {
        setSuccess('Popup form deleted.');
        if (inspectForm?.id === id) setInspectForm(null);
        loadData();
      }
    } catch {
      setError('Failed to delete form.');
    }
  };

  const handleInspectSubmissions = async (form: PopupForm) => {
    setInspectForm(form);
    try {
      const res = await fetch(`${base}/popups/${form.id}/submissions`);
      if (res.ok) {
        const data = await res.json();
        setSubmissions(data || []);
      }
    } catch {
      setError('Failed to load submissions.');
    }
  };

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canManage) return;
    setError('');
    setSuccess('');
    try {
      const res = await fetch(`${base}/settings`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          apiKeyConfigured: true,
          serverPrefix,
          defaultListId,
          doubleOptIn,
          syncEnabled,
        }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to update Mailchimp settings.');
      }
      setSuccess('Mailchimp integration settings updated.');
      loadData();
    } catch (err: unknown) {
      if (err instanceof Error) setError(err.message);
      else setError('Error updating settings.');
    }
  };

  return (
    <section className="dealership-panel" style={{ marginTop: '1.5rem', border: '1px solid var(--glass-line)', padding: '1.25rem', borderRadius: '8px' }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem', borderBottom: '1px solid var(--glass-line)', paddingBottom: '0.75rem' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '1.25rem' }}>Mailchimp</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            Customers who agree to hear from you are added to your Mailchimp audience.
          </p>
        </div>
      </div>

      {error && <div style={{ color: 'var(--error)', background: 'color-mix(in srgb, var(--error) 16%, transparent)', padding: '0.5rem', borderRadius: '4px', marginBottom: '1rem' }}>{error}</div>}
      {success && <div style={{ color: 'var(--success)', background: 'color-mix(in srgb, var(--success) 16%, transparent)', padding: '0.5rem', borderRadius: '4px', marginBottom: '1rem' }}>{success}</div>}
      {loading && <p>Loading marketing data...</p>}

      {tab === 'popups' && (
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <h3 style={{ margin: 0 }}>Configured Popup Modals</h3>
            {canManage && (
              <button type="button" onClick={() => setCreating(!creating)} style={{ padding: '0.4rem 0.8rem' }}>
                {creating ? 'Cancel' : '+ New Popup Form'}
              </button>
            )}
          </div>

          {creating && (
            <form onSubmit={handleCreateForm} style={{ background: 'var(--glass-strong)', padding: '1rem', borderRadius: '6px', marginBottom: '1.5rem', border: '1px solid var(--glass-line)' }}>
              <h4 style={{ margin: '0 0 0.75rem 0' }}>Create New Lead Capture Popup</h4>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '0.75rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Internal Form Name</label>
                  <input
                    type="text"
                    required
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                    placeholder="e.g. Spring 2026 Special"
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Modal Headline</label>
                  <input
                    type="text"
                    value={formHeadline}
                    onChange={(e) => setFormHeadline(e.target.value)}
                    placeholder="e.g. VIP Detailing Offer"
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
              </div>
              <div style={{ marginBottom: '0.75rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Modal Title / Hook</label>
                <input
                  type="text"
                  required
                  value={formTitle}
                  onChange={(e) => setFormTitle(e.target.value)}
                  placeholder="e.g. Get 15% Off Your First Ceramic Coating"
                  style={{ width: '100%', padding: '0.4rem' }}
                />
              </div>
              <div style={{ marginBottom: '0.75rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Description / Offer Terms</label>
                <textarea
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  rows={2}
                  placeholder="e.g. Join the Dapper VIP list to receive seasonal vehicle maintenance tips and private promotions."
                  style={{ width: '100%', padding: '0.4rem' }}
                />
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '1rem', marginBottom: '0.75rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Display Delay (seconds)</label>
                  <input
                    type="number"
                    min={0}
                    max={120}
                    value={formDelay}
                    onChange={(e) => setFormDelay(Number(e.target.value))}
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Cooldown (days between views)</label>
                  <input
                    type="number"
                    min={0}
                    max={365}
                    value={formCooldown}
                    onChange={(e) => setFormCooldown(Number(e.target.value))}
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Target Pages (comma-separated)</label>
                  <input
                    type="text"
                    value={formPages}
                    onChange={(e) => setFormPages(e.target.value)}
                    placeholder="/, /booking*"
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '0.75rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Mailchimp List / Audience ID</label>
                  <input
                    type="text"
                    value={formAudienceId}
                    onChange={(e) => setFormAudienceId(e.target.value)}
                    placeholder="Leave blank for default audience"
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Subscriber Tags (comma-separated)</label>
                  <input
                    type="text"
                    value={formTags}
                    onChange={(e) => setFormTags(e.target.value)}
                    placeholder="SpringPromo, WebLead"
                    style={{ width: '100%', padding: '0.4rem' }}
                  />
                </div>
              </div>
              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Success Message</label>
                <input
                  type="text"
                  value={formSuccessMessage}
                  onChange={(e) => setFormSuccessMessage(e.target.value)}
                  style={{ width: '100%', padding: '0.4rem' }}
                />
              </div>
              <button type="submit" style={{ padding: '0.5rem 1rem', background: '#d10f1f', color: '#fff', border: 'none', borderRadius: '4px' }}>
                Save &amp; Activate Popup
              </button>
            </form>
          )}

          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            {popups.map((form) => (
              <div
                key={form.id}
                style={{
                  border: '1px solid var(--glass-line)',
                  padding: '1rem',
                  borderRadius: '6px',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  background: form.active ? 'var(--glass-strong)' : 'var(--glass-line)',
                  opacity: form.active ? 1 : 0.75,
                }}
              >
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <strong>{form.name}</strong>
                    <span style={{ fontSize: '0.75rem', padding: '0.2rem 0.5rem', borderRadius: '12px', background: form.active ? 'color-mix(in srgb, var(--success) 16%, transparent)' : 'color-mix(in srgb, var(--error) 16%, transparent)', color: form.active ? 'var(--success)' : 'var(--error)' }}>
                      {form.active ? 'Active' : 'Disabled'}
                    </span>
                  </div>
                  <div style={{ fontSize: '0.9rem', color: 'var(--text)', marginTop: '0.25rem' }}>
                    Title: <em>&ldquo;{form.title}&rdquo;</em>
                  </div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
                    Delay: {form.displayDelaySeconds}s | Pages: {form.selectedPages.join(', ')} | Cooldown: {form.repeatDisplayLimitDays}d
                    {form.tags?.length > 0 && ` | Tags: ${form.tags.join(', ')}`}
                  </div>
                </div>

                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <button type="button" onClick={() => handleInspectSubmissions(form)} style={{ padding: '0.3rem 0.6rem', fontSize: '0.85rem' }}>
                    View Submissions
                  </button>
                  {canManage && (
                    <>
                      <button type="button" onClick={() => handleToggleActive(form)} style={{ padding: '0.3rem 0.6rem', fontSize: '0.85rem' }}>
                        {form.active ? 'Disable' : 'Enable'}
                      </button>
                      <button type="button" onClick={() => handleDeleteForm(form.id)} style={{ padding: '0.3rem 0.6rem', fontSize: '0.85rem', color: 'var(--error)' }}>
                        Delete
                      </button>
                    </>
                  )}
                </div>
              </div>
            ))}
            {popups.length === 0 && <p style={{ color: 'var(--text-muted)' }}>No popup forms configured for this location.</p>}
          </div>

          {inspectForm && (
            <div style={{ marginTop: '1.5rem', padding: '1rem', border: '1px solid var(--glass-line)', borderRadius: '6px', background: 'var(--glass-strong)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
                <h4 style={{ margin: 0 }}>Submissions for &ldquo;{inspectForm.name}&rdquo;</h4>
                <button type="button" onClick={() => setInspectForm(null)} style={{ padding: '0.2rem 0.5rem' }}>Close</button>
              </div>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                <thead>
                  <tr style={{ background: 'var(--glass-strong)', textAlign: 'left' }}>
                    <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Email</th>
                    <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Name</th>
                    <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>IP Address</th>
                    <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Date Submitted</th>
                  </tr>
                </thead>
                <tbody>
                  {submissions.map((sub) => (
                    <tr key={sub.id}>
                      <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>{sub.email}</td>
                      <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>{`${sub.firstName} ${sub.lastName}`.trim() || '—'}</td>
                      <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>{sub.ipAddress || '—'}</td>
                      <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>{new Date(sub.submittedAt).toLocaleString()}</td>
                    </tr>
                  ))}
                  {submissions.length === 0 && (
                    <tr>
                      <td colSpan={4} style={{ padding: '0.8rem', textAlign: 'center', color: 'var(--text-muted)' }}>No submissions recorded yet for this form.</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {tab === 'consents' && (
        <div>
          <h3 style={{ margin: '0 0 0.5rem 0' }}>CASL / CAN-SPAM Marketing Consent Ledger</h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
            Canadian Anti-Spam Legislation requires tracking express opt-in consent, date/time, IP address, and opt-out timestamps.
          </p>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ background: 'var(--glass-strong)', textAlign: 'left' }}>
                <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Email</th>
                <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Status</th>
                <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Source</th>
                <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Consent Wording</th>
                <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Sync Status</th>
                <th style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>Consented At</th>
              </tr>
            </thead>
            <tbody>
              {consents.map((c) => (
                <tr key={c.id}>
                  <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>
                    <strong>{c.email}</strong>
                    {c.firstName && <span style={{ color: 'var(--text-muted)', marginLeft: '0.3rem' }}>({c.firstName} {c.lastName})</span>}
                  </td>
                  <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>
                    <span style={{
                      padding: '0.15rem 0.4rem',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      background: c.consentStatus === 'opt_in' ? 'color-mix(in srgb, var(--success) 16%, transparent)' : 'color-mix(in srgb, var(--error) 16%, transparent)',
                      color: c.consentStatus === 'opt_in' ? 'var(--success)' : 'var(--error)',
                    }}>
                      {c.consentStatus}
                    </span>
                  </td>
                  <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>{c.consentSource}</td>
                  <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={c.consentText}>
                    {c.consentText}
                  </td>
                  <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>
                    <span style={{
                      padding: '0.15rem 0.4rem',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      background: c.syncStatus === 'synced' ? 'var(--neutral-chip)' : c.syncStatus === 'failed' ? 'color-mix(in srgb, var(--error) 16%, transparent)' : 'var(--gold-tint)',
                      color: c.syncStatus === 'synced' ? 'var(--text)' : c.syncStatus === 'failed' ? 'var(--error)' : 'var(--on-gold-tint)',
                    }}>
                      {c.syncStatus}
                    </span>
                  </td>
                  <td style={{ padding: '0.4rem', border: '1px solid var(--glass-line)' }}>{new Date(c.consentedAt).toLocaleDateString()}</td>
                </tr>
              ))}
              {consents.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ padding: '0.8rem', textAlign: 'center', color: 'var(--text-muted)' }}>No consent records found.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {tab === 'settings' && (
        <div>
          <h3 style={{ margin: '0 0 0.5rem 0' }}>Mailchimp settings</h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
            Your Mailchimp server prefix (the part before .api.mailchimp.com, e.g. us21), the audience to add people to, and whether Mailchimp sends a confirmation email first.
          </p>

          <form onSubmit={handleSaveSettings} style={{ maxWidth: '500px', background: 'var(--glass-strong)', padding: '1rem', borderRadius: '6px', border: '1px solid var(--glass-line)' }}>
            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>API Key Status</label>
              <div style={{ fontSize: '0.9rem', color: settings?.apiKeyConfigured ? 'var(--success)' : 'var(--error)' }}>
                {settings?.apiKeyConfigured ? '✓ Configured via Server Environment' : '⚠ API Key Not Configured'}
              </div>
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Server Prefix (Datacenter)</label>
              <input
                type="text"
                required
                value={serverPrefix}
                onChange={(e) => setServerPrefix(e.target.value)}
                placeholder="e.g. us21"
                style={{ width: '100%', padding: '0.4rem' }}
              />
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>Default Audience / List ID</label>
              <input
                type="text"
                value={defaultListId}
                onChange={(e) => setDefaultListId(e.target.value)}
                placeholder="e.g. 5a1b2c3d4e"
                style={{ width: '100%', padding: '0.4rem' }}
              />
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem' }}>
                <input
                  type="checkbox"
                  checked={doubleOptIn}
                  onChange={(e) => setDoubleOptIn(e.target.checked)}
                />
                Require Double Opt-In (Mailchimp sends confirmation email)
              </label>
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem' }}>
                <input
                  type="checkbox"
                  checked={syncEnabled}
                  onChange={(e) => setSyncEnabled(e.target.checked)}
                />
                Enable Automatic Subscriber Sync
              </label>
            </div>

            {canManage && (
              <button type="submit" style={{ padding: '0.5rem 1rem', background: '#d10f1f', color: '#fff', border: 'none', borderRadius: '4px' }}>
                Save Settings
              </button>
            )}
          </form>
        </div>
      )}
    </section>
  );
}
