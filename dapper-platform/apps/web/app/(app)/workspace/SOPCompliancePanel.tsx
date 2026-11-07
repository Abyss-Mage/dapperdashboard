'use client';
import {useCallback, useEffect, useState} from 'react';
import Select from '../GlassSelect';
import ConfirmDialog from './ConfirmDialog';

type SOPDocument = {
  id: string;
  title: string;
  category: 'safety' | 'chemical' | 'procedure' | 'onboarding';
  fileName: string;
  sha256Hash: string;
  version: number;
  pageCount: number;
  effectiveDate: string;
};

type SOPAcknowledgment = {
  id: string;
  sopDocumentId: string;
  documentTitle?: string;
  employeeId: string;
  actorId: string;
  acknowledgedSha256: string;
  trainingTrack: 'orientation' | 'assisted' | 'independent' | 'certified';
  notes: string;
  acknowledgedAt: string;
};

export default function SOPCompliancePanel({
  base,
  canAcknowledge,
  canManage = false,
}: {
  base: string;
  canAcknowledge: boolean;
  canManage?: boolean;
}) {
  const [docs, setDocs] = useState<SOPDocument[]>([]);
  const [acknowledgments, setAcknowledgments] = useState<SOPAcknowledgment[]>([]);
  const [selectedDoc, setSelectedDoc] = useState<SOPDocument | null>(null);
  const [trainingTrack, setTrainingTrack] = useState<'orientation' | 'assisted' | 'independent' | 'certified'>('orientation');
  const [notes, setNotes] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const loadData = useCallback(async () => {
    if (!base) return;
    setLoading(true);
    setError('');
    try {
      const [docRes, ackRes] = await Promise.all([
        fetch(`${base}/sops`),
        fetch(`${base}/sops/acknowledgments`),
      ]);
      if (!docRes.ok) throw new Error('Could not load SOP compliance documents');
      const docData = await docRes.json();
      setDocs(docData.items || []);

      if (ackRes.ok) {
        const ackData = await ackRes.json();
        setAcknowledgments(ackData.items || []);
      }
    } catch (err: any) {
      setError(err.message || 'Error loading SOP compliance');
    } finally {
      setLoading(false);
    }
  }, [base]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Administrators add a PDF (the file itself is the request body) or take one off the employee list.
  const [addTitle, setAddTitle] = useState('');
  const [addCategory, setAddCategory] = useState<SOPDocument['category']>('procedure');
  const [addFile, setAddFile] = useState<File | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const [removeTarget, setRemoveTarget] = useState<SOPDocument | null>(null);
  const [managing, setManaging] = useState(false);

  const addDocument = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!addFile) { setError('Choose a PDF file first.'); return; }
    setManaging(true); setError(''); setSuccess('');
    try {
      const q = new URLSearchParams({ title: addTitle.trim(), category: addCategory });
      const res = await fetch(`${base}/sops?${q}`, { method: 'POST', headers: { 'Content-Type': 'application/pdf' }, body: addFile });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error ?? 'The document could not be added.');
      setSuccess(`Added "${body.title}". Employees can now read and acknowledge it.`);
      setAddTitle(''); setAddFile(null); setFileKey((k) => k + 1);
      await loadData();
    } catch (err: any) { setError(err.message); }
    finally { setManaging(false); }
  };
  const removeDocument = async () => {
    if (!removeTarget) return;
    setManaging(true); setError(''); setSuccess('');
    try {
      const res = await fetch(`${base}/sops/${removeTarget.id}`, { method: 'DELETE' });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error ?? 'The document could not be removed.');
      setSuccess(`Removed "${removeTarget.title}" from the employee list. Earlier acknowledgments are kept.`);
      setRemoveTarget(null);
      await loadData();
    } catch (err: any) { setError(err.message); setRemoveTarget(null); }
    finally { setManaging(false); }
  };

  const handleAcknowledge = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedDoc) return;

    setLoading(true);
    setError('');
    setSuccess('');
    try {
      const res = await fetch(`${base}/sops/${selectedDoc.id}/acknowledge`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          trainingTrack,
          hash: selectedDoc.sha256Hash,
          notes: notes.trim(),
        }),
      });

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || errData.errors?.sop || 'Failed to record acknowledgment');
      }

      setSuccess(`Recorded verified acknowledgment for ${selectedDoc.title} (${trainingTrack} track)!`);
      setSelectedDoc(null);
      setNotes('');
      await loadData();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const getCategoryBadge = (cat: SOPDocument['category']) => {
    switch (cat) {
      case 'safety':
        return <span className="status-badge error">Safety</span>;
      case 'onboarding':
        return <span className="status-badge warning">Onboarding</span>;
      case 'procedure':
        return <span className="status-badge success">Procedure</span>;
      default:
        return <span className="status-badge neutral">{cat}</span>;
    }
  };

  const getTrackBadge = (track: SOPAcknowledgment['trainingTrack']) => {
    switch (track) {
      case 'certified':
        return <span className="status-badge success">Certified</span>;
      case 'independent':
        return <span className="status-badge warning">Independent</span>;
      case 'assisted':
        return <span className="status-badge neutral">Assisted</span>;
      case 'orientation':
        return <span className="status-badge pending">Orientation</span>;
      default:
        return <span className="status-badge">{track}</span>;
    }
  };

  return (
    <section className="panel detail-card">
      <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.75rem', marginBottom: '1rem'}}>
        <div>
          <h2>Standard Operating Procedures (SOP) & Compliance</h2>
        </div>
        <button className="secondary" onClick={loadData} disabled={loading}>
          Refresh
        </button>
      </div>

      {error && <div className="notice-banner error" style={{marginBottom: '1rem'}}>{error}</div>}
      {success && <div className="notice-banner success" style={{marginBottom: '1rem'}}>{success}</div>}

      {canManage && (
        <form onSubmit={addDocument} style={{background: 'var(--surface-sunken)', padding: '1rem', borderRadius: '8px', marginBottom: '1.25rem', border: '1px solid var(--border-color)', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))', gap: '0.75rem', alignItems: 'end'}}>
          <label>Title<input value={addTitle} onChange={(e) => setAddTitle(e.target.value)} minLength={3} maxLength={150} required placeholder="e.g. Chemical handling" style={{width: '100%'}} /></label>
          <label>Category
            <Select value={addCategory} onChange={(e: any) => setAddCategory(e.target.value)}>
              <option value="procedure">Procedure</option><option value="safety">Safety</option><option value="chemical">Chemical</option><option value="onboarding">Onboarding</option>
            </Select>
          </label>
          <label>PDF file (up to 10 MB)<input key={fileKey} type="file" accept="application/pdf,.pdf" required onChange={(e) => setAddFile(e.target.files?.[0] ?? null)} style={{width: '100%'}} /></label>
          <button type="submit" className="primary" disabled={managing || !addFile || addTitle.trim().length < 3}>{managing ? 'Adding…' : 'Add document'}</button>
        </form>
      )}

      {/* Acknowledgment Modal */}
      {selectedDoc && (
        <form onSubmit={handleAcknowledge} className="inline-form" style={{background: 'var(--surface-sunken)', padding: '1.25rem', borderRadius: '8px', marginBottom: '1.5rem', border: '1px solid var(--border-color)'}}>
          <h3 style={{margin: 0}}>Acknowledge SOP Training & Compliance</h3>
          <p style={{fontSize: '0.9rem', color: 'var(--text-muted)', margin: '0.25rem 0 1rem 0'}}>
            Document: <strong>{selectedDoc.title}</strong> · {selectedDoc.pageCount} Pages · SHA-256: <code>{selectedDoc.sha256Hash.slice(0, 16)}...</code>
          </p>

          <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '1rem', marginBottom: '1rem'}}>
            <div>
              <label>Training Stage / Track *</label>
              <Select
                value={trainingTrack}
                onChange={(e: any) => setTrainingTrack(e.target.value)}
                style={{width: '100%', padding: '0.5rem'}}
              >
                <option value="orientation">Phase 1: Orientation & Policy Review</option>
                <option value="assisted">Phase 2: Assisted Practical Observation</option>
                <option value="independent">Phase 3: Independent Supervised Execution</option>
                <option value="certified">Phase 4: Fully Certified Sign-Off</option>
              </Select>
            </div>
            <div>
              <label>Supervisor / Employee Notes</label>
              <input
                type="text"
                placeholder="Training observations, chemical safety equipment used..."
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                style={{width: '100%', padding: '0.5rem'}}
              />
            </div>
          </div>

          <div style={{display: 'flex', gap: '0.5rem', justifyContent: 'flex-end'}}>
            <button type="button" className="secondary" onClick={() => setSelectedDoc(null)}>Cancel</button>
            <button type="submit" className="primary" disabled={loading}>
              {loading ? 'Recording...' : 'Submit Verified Acknowledgment'}
            </button>
          </div>
        </form>
      )}

      {/* Documents Grid */}
      <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '1rem', marginBottom: '2rem'}}>
        {docs.map((doc) => (
          <div key={doc.id} style={{background: 'var(--surface-sunken)', padding: '1rem', borderRadius: '8px', border: '1px solid var(--border-color)'}}>
            <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem'}}>
              <strong style={{fontSize: '1.05rem'}}>{doc.title}</strong>
              {getCategoryBadge(doc.category)}
            </div>
            <div style={{fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '0.75rem'}}>
              <div>File: <code>{doc.fileName}</code></div>
              <div>Pages: {doc.pageCount} · Revision: v{doc.version}</div>
              <div style={{wordBreak: 'break-all', fontSize: '0.75rem', marginTop: '0.25rem'}}>
                SHA-256: <code>{doc.sha256Hash}</code>
              </div>
            </div>
            <div style={{display: 'flex', gap: '0.5rem', marginBottom: canAcknowledge ? '0.5rem' : 0}}>
              <a className="secondary" href={`${base}/sops/${doc.id}/file`} target="_blank" rel="noreferrer" style={{flex: 1, textAlign: 'center', fontSize: '0.85rem', padding: '0.45rem'}}>Open PDF</a>
              {canManage && <button type="button" className="secondary" style={{flex: 1, fontSize: '0.85rem'}} onClick={() => setRemoveTarget(doc)}>Remove</button>}
            </div>
            {canAcknowledge && (
              <button
                className="secondary"
                style={{width: '100%', fontSize: '0.85rem'}}
                onClick={() => {
                  setSelectedDoc(doc);
                  setNotes('');
                }}
              >
                Sign / Acknowledge Stage
              </button>
            )}
          </div>
        ))}
      </div>

      {!docs.length && !loading && <p style={{color: 'var(--text-muted)', marginBottom: '1.5rem'}}>No documents yet.{canManage ? ' Add the first PDF above.' : ''}</p>}
      <ConfirmDialog isOpen={removeTarget !== null} title="Remove document" entityName={removeTarget?.title ?? ''} entityType="Document" actionType="delete" isBusy={managing}
        explanation="Employees will no longer see this document. Acknowledgments already recorded are kept as evidence."
        onConfirm={() => void removeDocument()} onCancel={() => setRemoveTarget(null)} />

      {/* Completed Acknowledgments List */}
      <div>
        <h3 style={{margin: '0 0 0.5rem 0'}}>Employee Compliance & Acknowledgment Log</h3>
        <div className="management-list">
          {acknowledgments.map((ack) => (
            <article key={ack.id} style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem', padding: '0.6rem 0.8rem'}}>
              <div>
                <div style={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
                  <strong>{ack.documentTitle || 'SOP Document'}</strong>
                  {getTrackBadge(ack.trainingTrack)}
                </div>
                <small style={{display: 'block', color: 'var(--text-muted)'}}>
                  Recorded: {new Date(ack.acknowledgedAt).toLocaleString()} · Signer ID: {ack.actorId.slice(0, 8)}
                </small>
                {ack.notes && <small style={{fontStyle: 'italic'}}>{ack.notes}</small>}
              </div>
              <div style={{textAlign: 'right'}}>
                <span className="status-badge success" style={{fontSize: '0.75rem'}}>Verified Hash Match</span>
              </div>
            </article>
          ))}
          {!acknowledgments.length && (
            <p style={{padding: '1rem', textAlign: 'center', color: 'var(--text-muted)'}}>
              No training acknowledgments recorded for this location yet.
            </p>
          )}
        </div>
      </div>
    </section>
  );
}
