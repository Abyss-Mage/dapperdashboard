'use client';
import {useCallback, useEffect, useRef, useState} from 'react';
import InvoiceDraftEditor from './InvoiceDraftEditor';
import InvoiceDraftHistory from './InvoiceDraftHistory';
import SquareCardPayment from './SquareCardPayment';
import { printInvoiceDocument, usePrintTemplate } from './InvoicePrint';

type InvoiceItem = {
  id: string;
  description: string;
  quantity: number;
  unitPriceCents: number;
  totalPriceCents: number;
};

type Payment = {
  id: string;
  amountCents: number;
  currency: string;
  method: string;
  status: string;
  providerReference: string;
  notes: string;
  recordedAt: string;
};

type InvoiceEvent = {
  id: string;
  fromStatus: string | null;
  toStatus: string;
  notes: string;
  createdAt: string;
};

type Invoice = {
  version: number;
  issuerSnapshot?: {legalName:string;address:string;taxRegistration:string;currency:string;numberPrefix:string;version:number};
  id: string;
  invoiceNumber: string;
  customerId: string;
  customerName?: string;
  customerEmail?: string;
  status: 'draft' | 'issued' | 'partially_paid' | 'paid' | 'void';
  currency: string;
  subtotalCents: number;
  taxCents: number;
  totalCents: number;
  paidCents: number;
  dueCents: number;
  notes: string;
  issuedAt?: string;
  paidAt?: string;
  createdAt: string;
  items?: InvoiceItem[];
  payments?: Payment[];
  events?: InvoiceEvent[];
};

type Customer = {
  id: string;
  name: string;
  email: string;
};

type LineDraft = {
  description: string;
  quantity: number;
  unitPrice: string; // string for input handling, e.g. "120.00"
};

function parseMoney(value: string): number | null {
  if (!/^\d+(\.\d{1,2})?$/.test(value.trim())) return null;
  const [whole, fraction = ''] = value.trim().split('.');
  const cents = Number(whole + fraction.padEnd(2, '0'));
  return Number.isSafeInteger(cents) ? cents : null;
}

export default function InvoiceManager({
  base,
  canCreate,
  canManage,
  canRecordPayment,
  canIssue,
}: {
  base: string;
  canCreate: boolean;
  canManage: boolean;
  canRecordPayment: boolean;
  canIssue: boolean;
}) {
  const printTemplate = usePrintTemplate(`${base.split('/locations/')[0]}/invoice-template`);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [statusFilter, setStatusFilter] = useState('all');
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Creation state
  const [showCreate, setShowCreate] = useState(false);
  const [selectedCustomerId, setSelectedCustomerId] = useState('');
  const [customInvoiceNumber, setCustomInvoiceNumber] = useState('');
  const [notes, setNotes] = useState('');
  const [taxAmount, setTaxAmount] = useState('');
  const [lines, setLines] = useState<LineDraft[]>([
    {description: '', quantity: 1, unitPrice: ''},
  ]);

  // Selected Detail Modal state
  const [activeInvoice, setActiveInvoice] = useState<Invoice | null>(null);
  const [editingDraft, setEditingDraft] = useState(false);
  const invoiceDialog = useRef<HTMLDialogElement>(null);
  const invoiceTrigger = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    if (activeInvoice && invoiceDialog.current && !invoiceDialog.current.open) invoiceDialog.current.showModal();
    if (!activeInvoice && invoiceTrigger.current?.isConnected) invoiceTrigger.current.focus();
  }, [activeInvoice]);
  const [issuancePreview, setIssuancePreview] = useState<{id:string;legalName:string;address:string;taxRegistration:string;currency:string;numberPrefix:string;version:number}|null>(null);
  const [paymentAmount, setPaymentAmount] = useState('');
  const paymentMethod = 'cash';
  const [paymentNotes, setPaymentNotes] = useState('');
  const [busyPayment, setBusyPayment] = useState(false);
  const [voidNotes, setVoidNotes] = useState('');

  const formatCAD = (cents: number) => {
    if (!Number.isSafeInteger(cents)) return 'Enter valid amounts';
    return new Intl.NumberFormat('en-CA', {
      style: 'currency',
      currency: 'CAD',
    }).format(cents / 100);
  };

  const loadInvoices = useCallback(async () => {
    if (!base) return;
    setLoading(true);
    setError('');
    try {
      const res = await fetch(`${base}/invoices`);
      if (!res.ok) throw new Error('Could not load invoices');
      const data = await res.json();
      setInvoices(data.items || []);
    } catch (err: any) {
      setError(err.message || 'Error loading invoices');
    } finally {
      setLoading(false);
    }
  }, [base]);

  const loadCustomers = useCallback(async () => {
    if (!base) return;
    try {
      const res = await fetch(`${base}/customers`);
      if (res.ok) {
        const data = await res.json();
        setCustomers(data.items || []);
        if (data.items?.length && !selectedCustomerId) {
          setSelectedCustomerId(data.items[0].id);
        }
      }
    } catch {
      // non-fatal
    }
  }, [base, selectedCustomerId]);

  useEffect(() => {
    loadInvoices();
    loadCustomers();
  }, [loadInvoices, loadCustomers]);

  // Prints an already generated invoice straight from the list (the list rows lack the line items, so load the full invoice first).
  const printListedInvoice = async (id: string) => {
    setError('');
    try {
      const res = await fetch(`${base}/invoices/${id}`, {cache: 'no-store'});
      if (!res.ok) throw new Error('This invoice could not be opened for printing.');
      printInvoiceDocument(await res.json(), printTemplate);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'This invoice could not be opened for printing.');
    }
  };

  const loadInvoiceDetail = async (id: string) => {
    try {
      const res = await fetch(`${base}/invoices/${id}`);
      if (!res.ok) throw new Error('Could not load invoice detail');
      const full = await res.json();
      setActiveInvoice(full);
      setEditingDraft(false);
      setIssuancePreview(null);
      if (full.status === 'draft' && canIssue) {
        const previewResponse = await fetch(`${base}/invoices/${id}/issuance-preview`, {cache:'no-store'});
        const preview = await previewResponse.json();
        if (!previewResponse.ok) throw new Error(preview.error ?? 'Issuer review is unavailable.');
        setIssuancePreview(preview);
      }
      setPaymentAmount((full.dueCents / 100).toFixed(2));
      setPaymentNotes('');
    } catch (err: any) {
      setError(err.message);
    }
  };

  const calculateSubtotalCents = () => {
    return lines.reduce((acc, l) => {
      const qty = l.quantity;
      const priceCents = parseMoney(l.unitPrice);
      if (priceCents === null || !Number.isSafeInteger(qty) || qty <= 0) return NaN;
      const subtotal = acc + qty * priceCents;
      return Number.isSafeInteger(subtotal) ? subtotal : NaN;
    }, 0);
  };

  const calculateTaxCents = () => {
    return parseMoney(taxAmount) ?? NaN;
  };

  const handleAddLine = () => {
    setLines([...lines, {description: '', quantity: 1, unitPrice: ''}]);
  };

  const handleRemoveLine = (idx: number) => {
    if (lines.length <= 1) return;
    setLines(lines.filter((_, i) => i !== idx));
  };

  const handleLineChange = (idx: number, field: keyof LineDraft, value: any) => {
    const updated = [...lines];
    updated[idx] = {...updated[idx], [field]: value};
    setLines(updated);
  };

  const handleCreateInvoice = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCustomerId) {
      setError('Select a customer for this invoice');
      return;
    }
    const formattedItems = lines.map((l) => ({
      description: l.description.trim(),
      quantity: Number(l.quantity),
      unitPriceCents: parseMoney(l.unitPrice) ?? NaN,
    }));

    if (!Number.isSafeInteger(calculateSubtotalCents() + calculateTaxCents()) || formattedItems.some((i) => i.description.length < 2 || !Number.isSafeInteger(i.unitPriceCents) || i.unitPriceCents < 0 || !Number.isSafeInteger(i.quantity) || i.quantity <= 0)) {
      setError('Ensure all line items have a valid description and non-negative price');
      return;
    }

    setLoading(true);
    setError('');
    setSuccess('');
    try {
      const payload = {
        customerId: selectedCustomerId,
        invoiceNumber: customInvoiceNumber.trim() || undefined,
        taxCents: calculateTaxCents(),
        notes: notes.trim(),
        items: formattedItems,
      };

      const res = await fetch(`${base}/invoices`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || errData.errors?.invoice || 'Failed to create invoice');
      }

      const created = await res.json();
      setSuccess(`Invoice draft ${created.invoiceNumber} saved for review.`);
      setShowCreate(false);
      setCustomInvoiceNumber('');
      setNotes('');
      await loadInvoices();
      await loadInvoiceDetail(created.id);
    } catch (err: any) {
      setError(err.message || 'Failed to save invoice draft');
    } finally {
      setLoading(false);
    }
  };

  const handleRecordPayment = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!activeInvoice) return;
    const amountCents = parseMoney(paymentAmount);
    if (amountCents === null || amountCents <= 0) {
      setError('Enter a positive payment amount');
      return;
    }
    if (amountCents > activeInvoice.dueCents) {
      setError(`Payment cannot exceed the balance due of ${formatCAD(activeInvoice.dueCents)}`);
      return;
    }

    let receiptStorageKey = '';
    setBusyPayment(true);
    setError('');
    try {
      const fingerprintBytes = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify({amountCents,method:paymentMethod,notes:paymentNotes.trim()})));
      const fingerprint = Array.from(new Uint8Array(fingerprintBytes), (value) => value.toString(16).padStart(2, '0')).join('');
      receiptStorageKey = `dapper-cash-receipt:${base}:${activeInvoice.id}:${fingerprint}`;
      let idemKey = sessionStorage.getItem(receiptStorageKey);
      if (!idemKey) {idemKey = crypto.randomUUID(); sessionStorage.setItem(receiptStorageKey, idemKey);}
      const res = await fetch(`${base}/invoices/${activeInvoice.id}/payments`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          amountCents,
          method: paymentMethod,
          idempotencyKey: idemKey,
          notes: paymentNotes.trim(),
        }),
      });

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || errData.errors?.amountCents || 'Failed to record payment');
      }
	  sessionStorage.removeItem(receiptStorageKey);

      setSuccess(`Payment of ${formatCAD(amountCents)} recorded via ${paymentMethod.replace('_', ' ')}!`);
      await loadInvoices();
      await loadInvoiceDetail(activeInvoice.id);
    } catch (err: any) {
      setError(err.message || 'Payment recording failed');
    } finally {
      setBusyPayment(false);
    }
  };

  const issueReviewedInvoice = async () => {
    if (!activeInvoice || activeInvoice.status !== 'draft' || !canIssue || !issuancePreview) return;
    setLoading(true); setError(''); setSuccess('');
    try {
      const response = await fetch(`${base}/invoices/${activeInvoice.id}/issue`, {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({issuerId:issuancePreview.id,issuerVersion:issuancePreview.version,invoiceVersion:activeInvoice.version})});
      const body = await response.json(); if (!response.ok) throw new Error(body.error ?? 'Invoice could not be issued.');
      setSuccess(`Invoice ${body.invoiceNumber} issued.`); await loadInvoices(); await loadInvoiceDetail(body.id);
    } catch (err) {setError(err instanceof Error ? err.message : 'Invoice issuance failed.');}
    finally {setLoading(false);}
  };

  const handleVoidInvoice = async () => {
    if (!activeInvoice) return;
    if (!confirm(`Are you sure you want to void invoice ${activeInvoice.invoiceNumber}? This action cannot be undone.`)) {
      return;
    }
    setLoading(true);
    setError('');
    try {
      const res = await fetch(`${base}/invoices/${activeInvoice.id}/void`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({notes: voidNotes || 'Voided by operator'}),
      });
      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || 'Could not void invoice');
      }
      setSuccess(`Invoice ${activeInvoice.invoiceNumber} has been voided.`);
      await loadInvoices();
      await loadInvoiceDetail(activeInvoice.id);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const filteredInvoices = invoices.filter((inv) => {
    if (statusFilter !== 'all' && inv.status !== statusFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      const numMatch = inv.invoiceNumber.toLowerCase().includes(q);
      const custMatch = inv.customerName?.toLowerCase().includes(q) || inv.customerEmail?.toLowerCase().includes(q);
      if (!numMatch && !custMatch) return false;
    }
    return true;
  });

  const getStatusBadge = (status: Invoice['status']) => {
    switch (status) {
      case 'paid':
        return <span className="status-badge success">Paid</span>;
      case 'partially_paid':
        return <span className="status-badge warning">Partially Paid</span>;
      case 'issued':
        return <span className="status-badge pending">Issued</span>;
      case 'void':
        return <span className="status-badge neutral">Void</span>;
      default:
        return <span className="status-badge">{status}</span>;
    }
  };

  return (
    <section className="panel detail-card invoice-manager">
      <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.75rem', marginBottom: '1rem'}}>
        <div>
          <h2>Invoices & Exact Payments (CAD)</h2>
          <p style={{margin: 0, fontSize: '0.875rem', color: 'var(--text-muted)'}}>
            Review invoice drafts, issue approved documents and record authorized cash receipts.
          </p>
        </div>
        <div style={{display: 'flex', gap: '0.5rem'}}>
          {canCreate && (
            <button className="primary" onClick={() => setShowCreate(!showCreate)}>
              {showCreate ? 'Close Form' : '+ Create Invoice Draft'}
            </button>
          )}
          <button className="secondary" onClick={loadInvoices} disabled={loading}>
            Refresh
          </button>
        </div>
      </div>

      {error && <div className="notice-banner error" style={{marginBottom: '1rem'}}>{error}</div>}
      {success && <div className="notice-banner success" style={{marginBottom: '1rem'}}>{success}</div>}

      {/* Creation Form Panel */}
      {showCreate && (
        <form onSubmit={handleCreateInvoice} className="inline-form" style={{display: 'flex', flexDirection: 'column', gap: '1rem', background: 'var(--surface-sunken)', padding: '1.25rem', borderRadius: '8px', marginBottom: '1.5rem', border: '1px solid var(--border-color)'}}>
          <h3 style={{margin: 0}}>Create itemized invoice draft</h3>

          <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '1rem'}}>
            <div>
              <label htmlFor="draft-customer-select">Customer *</label>
              <select
                id="draft-customer-select"
                aria-label="Customer"
                value={selectedCustomerId}
                onChange={(e) => setSelectedCustomerId(e.target.value)}
                required
                style={{width: '100%', padding: '0.5rem'}}
              >
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.email || 'No email'})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label htmlFor="draft-custom-number">Draft reference (optional, generated if blank)</label>
              <input
                id="draft-custom-number"
                aria-label="Draft reference"
                type="text"
                placeholder="INV-20261022-XXXX"
                value={customInvoiceNumber}
                onChange={(e) => setCustomInvoiceNumber(e.target.value)}
                style={{width: '100%', padding: '0.5rem'}}
              />
            </div>
            <div>
              <label htmlFor="draft-tax-amount">Reviewed tax amount (CAD)</label>
              <div style={{display: 'flex', gap: '0.5rem'}}>
                <input
                  id="draft-tax-amount"
                  aria-label="Reviewed tax amount (CAD)"
                  type="number"
                  step="0.01"
                  min="0"
                  required
                  value={taxAmount}
                  onChange={(e) => setTaxAmount(e.target.value)}
                  style={{width: '100px', padding: '0.5rem'}}
                />
              </div>
            </div>
          </div>

          <div>
            <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem'}}>
              <label style={{fontWeight: 600}}>Line Items *</label>
              <button type="button" className="secondary" onClick={handleAddLine} style={{padding: '0.25rem 0.5rem', fontSize: '0.8rem'}}>
                + Add Item
              </button>
            </div>
            {lines.map((line, idx) => (
              <div key={idx} style={{display: 'flex', gap: '0.5rem', alignItems: 'center', marginBottom: '0.5rem', flexWrap: 'wrap'}}>
                <label htmlFor={`draft-line-desc-${idx}`} style={{position: 'absolute', width: 1, height: 1, padding: 0, margin: -1, overflow: 'hidden', clip: 'rect(0,0,0,0)', whiteSpace: 'nowrap', border: 0}}>
                  Item {idx + 1} description
                </label>
                <input
                  id={`draft-line-desc-${idx}`}
                  aria-label={`Item ${idx + 1} description`}
                  type="text"
                  placeholder="Service description"
                  value={line.description}
                  onChange={(e) => handleLineChange(idx, 'description', e.target.value)}
                  required
                  style={{flex: 3, minWidth: '180px', padding: '0.5rem'}}
                />
                <div style={{display: 'flex', alignItems: 'center', gap: '0.25rem'}}>
                  <label htmlFor={`draft-line-qty-${idx}`} style={{fontSize: '0.85rem'}}>Qty:</label>
                  <input
                    id={`draft-line-qty-${idx}`}
                    aria-label={`Item ${idx + 1} quantity`}
                    type="number"
                    min="1"
                    value={line.quantity}
                    onChange={(e) => handleLineChange(idx, 'quantity', parseInt(e.target.value) || 1)}
                    required
                    style={{width: '60px', padding: '0.5rem'}}
                  />
                </div>
                <div style={{display: 'flex', alignItems: 'center', gap: '0.25rem'}}>
                  <label htmlFor={`draft-line-price-${idx}`} style={{fontSize: '0.85rem'}}>$</label>
                  <input
                    id={`draft-line-price-${idx}`}
                    aria-label={`Item ${idx + 1} unit price in CAD`}
                    type="number"
                    step="0.01"
                    min="0"
                    placeholder="0.00"
                    value={line.unitPrice}
                    onChange={(e) => handleLineChange(idx, 'unitPrice', e.target.value)}
                    required
                    style={{width: '100px', padding: '0.5rem'}}
                  />
                </div>
                <span style={{fontSize: '0.85rem', color: 'var(--text-muted)', minWidth: '80px'}}>
                  {formatCAD(Math.round(parseFloat(line.unitPrice || '0') * 100) * (line.quantity || 1))}
                </span>
                {lines.length > 1 && (
                  <button type="button" className="secondary" onClick={() => handleRemoveLine(idx)} style={{padding: '0.35rem 0.6rem', color: 'var(--color-danger)'}}>
                    ✕
                  </button>
                )}
              </div>
            ))}
          </div>

          <div style={{display: 'flex', justifyContent: 'flex-end', gap: '2rem', background: 'var(--surface-color)', padding: '0.75rem', borderRadius: '4px', border: '1px solid var(--border-color)'}}>
            <div>Subtotal: <strong>{formatCAD(calculateSubtotalCents())}</strong></div>
            <div>Reviewed tax: <strong>{formatCAD(calculateTaxCents())}</strong></div>
            <div style={{fontSize: '1.1rem', color: 'var(--brand-primary)'}}>Total Due: <strong>{formatCAD(calculateSubtotalCents() + calculateTaxCents())}</strong></div>
          </div>

          <div>
            <label htmlFor="draft-notes-textarea">Customer / Internal Notes</label>
            <textarea
              id="draft-notes-textarea"
              aria-label="Customer / Internal Notes"
              rows={2}
              placeholder="Payment instructions, warranty details, vehicle notes..."
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              style={{width: '100%', padding: '0.5rem'}}
            />
          </div>

          <div style={{display: 'flex', gap: '0.5rem', justifyContent: 'flex-end'}}>
            <button type="button" className="secondary" onClick={() => setShowCreate(false)}>Cancel</button>
            <button type="submit" className="primary" disabled={loading}>
              {loading ? 'Saving...' : 'Save Draft'}
            </button>
          </div>
        </form>
      )}

      {/* Filter and Search Bar */}
      <div style={{display: 'flex', gap: '0.75rem', marginBottom: '1rem', flexWrap: 'wrap'}}>
        <label htmlFor="invoice-search-filter" style={{position: 'absolute', width: 1, height: 1, padding: 0, margin: -1, overflow: 'hidden', clip: 'rect(0,0,0,0)', whiteSpace: 'nowrap', border: 0}}>
          Search invoices
        </label>
        <input
          id="invoice-search-filter"
          aria-label="Search by invoice number or customer name"
          type="text"
          placeholder="Search by invoice number or customer name..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{flex: 1, minWidth: '220px', padding: '0.5rem'}}
        />
        <label htmlFor="invoice-status-filter" style={{position: 'absolute', width: 1, height: 1, padding: 0, margin: -1, overflow: 'hidden', clip: 'rect(0,0,0,0)', whiteSpace: 'nowrap', border: 0}}>
          Filter by status
        </label>
        <select
          id="invoice-status-filter"
          aria-label="Filter invoices by status"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          style={{padding: '0.5rem', minWidth: '140px'}}
        >
          <option value="all">All Statuses</option>
          <option value="issued">Issued (Unpaid)</option>
          <option value="draft">Drafts for review</option>
          <option value="partially_paid">Partially Paid</option>
          <option value="paid">Paid in Full</option>
          <option value="void">Void</option>
        </select>
      </div>

      {/* Invoices List */}
      <div className="management-list">
        {filteredInvoices.map((inv) => (
          <article key={inv.id} style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.75rem', padding: '0.75rem 1rem'}}>
            <div>
              <div style={{display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem'}}>
                <strong style={{fontSize: '1.05rem', letterSpacing: '0.02em'}}>{inv.invoiceNumber}</strong>
                {getStatusBadge(inv.status)}
              </div>
              <small style={{display: 'block', color: 'var(--text-muted)'}}>
                Customer: <strong>{inv.customerName || 'Customer'}</strong> ({inv.customerEmail || 'No email'})
              </small>
              <small style={{color: 'var(--text-muted)'}}>
                Created: {new Date(inv.createdAt).toLocaleDateString()} · Currency: {inv.currency}
              </small>
            </div>
            <div style={{textAlign: 'right', display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: '0.25rem'}}>
              <div>
                <span style={{fontSize: '0.85rem', color: 'var(--text-muted)'}}>Total: </span>
                <strong>{formatCAD(inv.totalCents)}</strong>
              </div>
              <div>
                <span style={{fontSize: '0.85rem', color: inv.dueCents > 0 ? 'var(--color-danger)' : 'var(--color-success)'}}>
                  Balance Due: <strong>{formatCAD(inv.dueCents)}</strong>
                </span>
              </div>
              <div style={{display: 'flex', gap: '0.4rem', marginTop: '0.25rem'}}>
                <button
                  className="secondary"
                  style={{padding: '0.25rem 0.6rem', fontSize: '0.8rem'}}
                  onClick={(event) => {invoiceTrigger.current=event.currentTarget;loadInvoiceDetail(inv.id);}}
                >
                  View / Pay
                </button>
                {inv.status !== 'draft' && inv.status !== 'void' && (
                  <button
                    className="secondary"
                    style={{padding: '0.25rem 0.6rem', fontSize: '0.8rem'}}
                    onClick={() => void printListedInvoice(inv.id)}
                  >
                    Print
                  </button>
                )}
              </div>
            </div>
          </article>
        ))}
        {!filteredInvoices.length && !loading && (
          <p style={{padding: '1.5rem', textAlign: 'center', color: 'var(--text-muted)'}}>
            No invoices found matching criteria.
          </p>
        )}
      </div>

      {/* Invoice Detail & Payment Modal */}
      {activeInvoice && (
        <dialog ref={invoiceDialog} aria-label="Invoice detail" onCancel={() => setActiveInvoice(null)} style={{padding: 0, border: 0, background: 'transparent', width: 'calc(100% - 2rem)', maxWidth: '720px'}}>
          <div className="modal-content" style={{background: 'var(--surface-color)', maxWidth: '720px', width: '100%', maxHeight: '90vh', overflowY: 'auto', borderRadius: '10px', padding: '1.5rem', border: '1px solid var(--border-color)', position: 'relative'}}>
            <button
              onClick={() => setActiveInvoice(null)}
              aria-label="Close invoice"
              style={{position: 'absolute', top: '1rem', right: '1rem', background: 'transparent', border: 'none', fontSize: '1.25rem', cursor: 'pointer', color: 'var(--text-color)'}}
            >
              ✕
            </button>

            <div style={{borderBottom: '1px solid var(--border-color)', paddingBottom: '1rem', marginBottom: '1rem'}}>
              <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                <div>
                  <h3 style={{margin: 0, fontSize: '1.4rem', overflowWrap: 'anywhere', paddingRight: '2rem'}}>{activeInvoice.invoiceNumber}</h3>
                  <small style={{color: 'var(--text-muted)'}}>{activeInvoice.issuedAt ? `Issued: ${new Date(activeInvoice.issuedAt).toLocaleString()}` : `Draft created: ${new Date(activeInvoice.createdAt).toLocaleString()}`}</small>
                </div>
                <div>{getStatusBadge(activeInvoice.status)}</div>
              </div>
              <div style={{marginTop: '0.5rem', fontSize: '0.9rem'}}>
                <div>Bill To: <strong>{activeInvoice.customerName}</strong></div>
                {activeInvoice.customerEmail && <div>Email: {activeInvoice.customerEmail}</div>}
                {activeInvoice.notes && <div style={{marginTop: '0.25rem', fontStyle: 'italic'}}>Notes: {activeInvoice.notes}</div>}
                {activeInvoice.issuerSnapshot && <div style={{marginTop:'1rem'}}><strong>{activeInvoice.issuerSnapshot.legalName}</strong><p style={{whiteSpace:'pre-line'}}>{activeInvoice.issuerSnapshot.address}</p>{activeInvoice.issuerSnapshot.taxRegistration && <p>Tax registration: {activeInvoice.issuerSnapshot.taxRegistration}</p>}</div>}
              </div>
            </div>

            {/* Line Items Table */}
            {error && <div role="alert" className="notice-banner error">{error}</div>}
            {activeInvoice.status === 'draft' && canManage && !editingDraft && <button className="secondary" onClick={() => {setError('');setEditingDraft(true);}}>Edit draft</button>}
            {editingDraft && activeInvoice.status === 'draft' && canManage && <InvoiceDraftEditor key={`${activeInvoice.id}:${activeInvoice.version}`} base={base} invoice={activeInvoice} onCancel={() => setEditingDraft(false)} onReload={() => loadInvoiceDetail(activeInvoice.id)} onSaved={async () => {setSuccess('Invoice draft changes saved. Review the updated amounts before issuing.'); await loadInvoices(); await loadInvoiceDetail(activeInvoice.id);}} />}
            <div style={{marginBottom: '1rem'}}>
              <h4 style={{margin: '0 0 0.5rem 0'}}>Itemized Services</h4>
              <table style={{width: '100%', borderCollapse: 'collapse', fontSize: '0.9rem'}}>
                <thead>
                  <tr style={{borderBottom: '1px solid var(--border-color)', textAlign: 'left'}}>
                    <th style={{padding: '0.5rem 0'}}>Description</th>
                    <th style={{padding: '0.5rem 0', textAlign: 'center'}}>Qty</th>
                    <th style={{padding: '0.5rem 0', textAlign: 'right'}}>Unit Price</th>
                    <th style={{padding: '0.5rem 0', textAlign: 'right'}}>Total</th>
                  </tr>
                </thead>
                <tbody>
                  {activeInvoice.items?.map((item) => (
                    <tr key={item.id} style={{borderBottom: '1px solid var(--surface-sunken)'}}>
                      <td style={{padding: '0.5rem 0'}}>{item.description}</td>
                      <td style={{padding: '0.5rem 0', textAlign: 'center'}}>{item.quantity}</td>
                      <td style={{padding: '0.5rem 0', textAlign: 'right'}}>{formatCAD(item.unitPriceCents)}</td>
                      <td style={{padding: '0.5rem 0', textAlign: 'right'}}><strong>{formatCAD(item.totalPriceCents)}</strong></td>
                    </tr>
                  ))}
                </tbody>
              </table>

              <div style={{marginTop: '0.75rem', padding: '0.75rem', background: 'var(--surface-sunken)', borderRadius: '6px', textAlign: 'right'}}>
                <div>Subtotal: <strong>{formatCAD(activeInvoice.subtotalCents)}</strong></div>
                <div>Tax: <strong>{formatCAD(activeInvoice.taxCents)}</strong></div>
                <div>Total: <strong>{formatCAD(activeInvoice.totalCents)}</strong></div>
                <div>Paid to Date: <strong style={{color: 'var(--color-success)'}}>{formatCAD(activeInvoice.paidCents)}</strong></div>
                <div style={{fontSize: '1.1rem', marginTop: '0.25rem', color: activeInvoice.dueCents > 0 ? 'var(--color-danger)' : 'var(--color-success)'}}>
                  Balance Due: <strong>{formatCAD(activeInvoice.dueCents)}</strong>
                </div>
              </div>
            </div>

            {/* Payment Recording Section */}
            {activeInvoice.status === 'draft' && <div className="panel detail-card"><p>Review the customer, line items, tax amount and proposed legal issuer. Issued details become immutable.</p>{issuancePreview && <div><strong>{issuancePreview.legalName}</strong><p style={{whiteSpace:'pre-line'}}>{issuancePreview.address}</p>{issuancePreview.taxRegistration && <p>Tax registration: {issuancePreview.taxRegistration}</p>}<p>{issuancePreview.currency} · Number prefix {issuancePreview.numberPrefix} · Issuer revision {issuancePreview.version}. The sequence number is assigned at issuance.</p></div>}{canIssue && <><button className="primary" disabled={loading||!issuancePreview||editingDraft} onClick={issueReviewedInvoice}>{loading?'Issuing…':'Issue reviewed invoice'}</button><button className="secondary" disabled={loading} onClick={()=>loadInvoiceDetail(activeInvoice.id)}>Refresh review</button></>}</div>}
            {activeInvoice.dueCents > 0 && ['issued','partially_paid'].includes(activeInvoice.status) && canRecordPayment && (
              <form onSubmit={handleRecordPayment} style={{background: 'var(--surface-sunken)', padding: '1rem', borderRadius: '8px', marginBottom: '1rem', border: '1px solid var(--border-color)'}}>
                <h4 style={{margin: '0 0 0.5rem 0'}}>Record cash received</h4>
                <p>This form records cash only. Card payments use the secure card form below once Square is set up.</p>
                <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '0.75rem', marginBottom: '0.5rem'}}>
                  <div>
                    <label htmlFor="cash-payment-amount" style={{fontSize: '0.85rem'}}>Amount ($CAD)</label>
                    <input
                      id="cash-payment-amount"
                      aria-label="Cash payment amount in CAD"
                      type="number"
                      step="0.01"
                      min="0.01"
                      max={(activeInvoice.dueCents / 100).toFixed(2)}
                      value={paymentAmount}
                      onChange={(e) => setPaymentAmount(e.target.value)}
                      required
                      style={{width: '100%', padding: '0.4rem'}}
                    />
                  </div>
                  <div>
                    <span style={{fontSize: '0.85rem', fontWeight: 600, display: 'block'}}>Payment Method</span>
                    <p style={{margin: '0.25rem 0'}}>Cash received in person</p>
                  </div>
                </div>
                <div>
                  <label htmlFor="cash-payment-notes" style={{fontSize: '0.85rem'}}>Notes</label>
                  <input
                    id="cash-payment-notes"
                    aria-label="Payment transaction remarks or teller notes"
                    type="text"
                    placeholder="Transaction remarks or teller note"
                    value={paymentNotes}
                    onChange={(e) => setPaymentNotes(e.target.value)}
                    style={{width: '100%', padding: '0.4rem'}}
                  />
                </div>
                <div style={{marginTop: '0.75rem', textAlign: 'right'}}>
                  <button type="submit" className="primary" disabled={busyPayment}>
                    {busyPayment ? 'Recording...' : `Record Payment of ${formatCAD(Math.round(parseFloat(paymentAmount || '0') * 100))}`}
                  </button>
                </div>
              </form>
            )}

            {activeInvoice.dueCents > 0 && ['issued','partially_paid'].includes(activeInvoice.status) && canRecordPayment && (
              <SquareCardPayment key={'card-' + activeInvoice.id} infoUrl={`${base}/square/binding`} payUrl={`${base}/invoices/${activeInvoice.id}/square-payments`} invoiceId={activeInvoice.id} dueCents={activeInvoice.dueCents} onPaid={async () => { await loadInvoices(); await loadInvoiceDetail(activeInvoice.id); }} />
            )}

            {/* Payments History */}
            {activeInvoice.payments && activeInvoice.payments.length > 0 && (
              <div style={{marginBottom: '1rem'}}>
                <h4 style={{margin: '0 0 0.5rem 0'}}>Payment History</h4>
                <div style={{fontSize: '0.85rem'}}>
                  {activeInvoice.payments.map((p) => (
                    <div key={p.id} style={{display: 'flex', justifyContent: 'space-between', padding: '0.4rem 0', borderBottom: '1px solid var(--surface-sunken)'}}>
                      <div>
                        <strong>{formatCAD(p.amountCents)}</strong> via {p.method.replace('_', ' ')}
                        {p.providerReference && <span style={{color: 'var(--text-muted)'}}> ({p.providerReference})</span>}
                        {p.notes && <div style={{color: 'var(--text-muted)', fontSize: '0.8rem'}}>{p.notes}</div>}
                      </div>
                      <div style={{color: 'var(--text-muted)'}}>
                        {new Date(p.recordedAt).toLocaleString()}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Audit Trail */}
            <InvoiceDraftHistory base={base} id={activeInvoice.id} version={activeInvoice.version} />
            {activeInvoice.events && activeInvoice.events.length > 0 && (
              <div style={{marginBottom: '1rem'}}>
                <h4 style={{margin: '0 0 0.5rem 0'}}>Audit Events</h4>
                <div style={{fontSize: '0.8rem', color: 'var(--text-muted)'}}>
                  {activeInvoice.events.map((ev) => (
                    <div key={ev.id} style={{padding: '0.2rem 0'}}>
                      • {new Date(ev.createdAt).toLocaleString()}: Status {ev.fromStatus ? `${ev.fromStatus} → ` : ''}<strong>{ev.toStatus}</strong> {ev.notes ? `(${ev.notes})` : ''}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Void / Actions footer */}
            <div style={{display: 'flex', flexWrap: 'wrap', gap: '.75rem', justifyContent: 'space-between', alignItems: 'center', marginTop: '1.5rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)'}}>
              <div>
                {canManage && activeInvoice.status !== 'void' && activeInvoice.paidCents === 0 && (
                  <div style={{display: 'flex', flexWrap: 'wrap', gap: '0.5rem', alignItems: 'center'}}>
                    <label htmlFor="void-reason-input" style={{position: 'absolute', width: 1, height: 1, padding: 0, margin: -1, overflow: 'hidden', clip: 'rect(0,0,0,0)', whiteSpace: 'nowrap', border: 0}}>
                      Reason to void invoice
                    </label>
                    <input
                      id="void-reason-input"
                      aria-label="Reason to void invoice"
                      type="text"
                      placeholder="Reason to void..."
                      value={voidNotes}
                      onChange={(e) => setVoidNotes(e.target.value)}
                      style={{padding: '0.35rem', fontSize: '0.85rem'}}
                    />
                    <button className="secondary" onClick={handleVoidInvoice} style={{color: 'var(--color-danger)'}}>
                      Void Invoice
                    </button>
                  </div>
                )}
              </div>
              <div style={{display: 'flex', gap: '0.5rem'}}>
                <button type="button" className="secondary" onClick={() => printInvoiceDocument(activeInvoice, printTemplate)}>
                  Print invoice
                </button>
                <button type="button" className="primary" onClick={() => setActiveInvoice(null)}>
                  Done
                </button>
              </div>
            </div>
          </div>
        </dialog>
      )}
    </section>
  );
}
