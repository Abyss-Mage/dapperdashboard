'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

type View = {
  smtp: { host: string; port: number; username: string; fromAddress: string; fromName: string; passwordSet: boolean; saved: boolean };
  sms: { accountSid: string; from: string; tokenSet: boolean; saved: boolean };
  emailSource: '' | 'saved' | 'server';
  emailWorking: boolean;
};
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
async function readJson(r: Response) {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'That could not be done.');
  return b;
}

/* Outgoing email (SMTP) and text messages (Twilio) for the whole business. Passwords and the Twilio Auth Token are kept encrypted on the
   server and are never shown again: a blank box means "keep the one I saved". */
export default function MessagingSettings({ organizationId, revision }: { organizationId: string; revision: number }) {
  const url = `/api/v1/organizations/${organizationId}/messaging`;
  const [view, setView] = useState<View | null>(null);
  const [host, setHost] = useState(''), [port, setPort] = useState('587'), [user, setUser] = useState(''), [pass, setPass] = useState(''), [from, setFrom] = useState(''), [fromName, setFromName] = useState('');
  const [sid, setSid] = useState(''), [token, setToken] = useState(''), [num, setNum] = useState('');
  const [mailTo, setMailTo] = useState(''), [smsTo, setSmsTo] = useState('');
  const [busy, setBusy] = useState(''), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [confirm, setConfirm] = useState<'smtp' | 'sms' | null>(null);

  const load = useCallback(async () => {
    const v: View = await readJson(await fetch(url, { cache: 'no-store' }));
    setView(v);
    setHost(v.smtp.host); setPort(String(v.smtp.port || 587)); setUser(v.smtp.username); setFrom(v.smtp.fromAddress); setFromName(v.smtp.fromName); setPass('');
    setSid(v.sms.accountSid); setNum(v.sms.from); setToken('');
  }, [url]);
  useEffect(() => { setError(''); load().catch((e) => setError(e.message)); }, [load, revision]);

  async function act(name: string, work: () => Promise<string>) {
    setBusy(name); setError(''); setNotice('');
    try { setNotice(await work()); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'That could not be done.'); }
    finally { setBusy(''); }
  }
  const saveSmtp = (e: React.FormEvent) => { e.preventDefault(); void act('smtp', async () => { await readJson(await fetch(url + '/smtp', send('PUT', { host, port: Number(port), username: user, password: pass, fromAddress: from, fromName }))); return 'Email settings saved and in use now.'; }); };
  const testMail = () => void act('mail-test', async () => { await readJson(await fetch(url + '/smtp/test', send('POST', { to: mailTo }))); return 'Test email sent to ' + mailTo + '. Check that inbox (and spam).'; });
  const saveSms = (e: React.FormEvent) => { e.preventDefault(); void act('sms', async () => { await readJson(await fetch(url + '/sms', send('PUT', { accountSid: sid, authToken: token, from: num }))); return 'Twilio settings saved.'; }); };
  const checkSms = () => void act('sms-check', async () => { await readJson(await fetch(url + '/sms/test', send('POST', { to: '' }))); return 'Twilio accepted the Account SID and Auth Token.'; });
  const testSms = () => void act('sms-test', async () => { await readJson(await fetch(url + '/sms/test', send('POST', { to: smsTo }))); return 'Test text sent to ' + smsTo + '.'; });
  async function remove() {
    const which = confirm; setConfirm(null);
    if (which) await act('remove', async () => { await readJson(await fetch(url + '/' + which, { method: 'DELETE' })); return which === 'smtp' ? 'Saved email settings removed.' : 'Twilio settings removed.'; });
  }

  const source = view?.emailSource;
  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Email &amp; SMS</h2>
        <p className={styles.personSub}>Where the system sends email (invitations, password resets) and text messages from. Only Super Admin and Owner roles can see or change this.</p>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {view && (
        <>
          <form onSubmit={saveSmtp} className={styles.resource}>
            <div className={styles.rowBetween}>
              <h3 style={{ margin: 0 }}>Email (SMTP)</h3>
              <span className={styles.personSub}>{source === 'saved' ? 'Using the settings saved here' : source === 'server' ? 'Using the server’s own settings' : 'Not set up: no email can be sent'}</span>
            </div>
            <div className={styles.inlineForm}>
              <label className="form-field">Email server<input value={host} onChange={(e) => setHost(e.target.value)} placeholder="smtp.example.com" maxLength={255} autoComplete="off" required disabled={!!busy} /></label>
              <label className="form-field" style={{ maxWidth: 160 }}>Port<input inputMode="numeric" pattern="[0-9]*" maxLength={5} value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))} disabled={!!busy} /></label>
            </div>
            <div className={styles.inlineForm}>
              <label className="form-field">Username<input value={user} onChange={(e) => setUser(e.target.value)} maxLength={255} autoComplete="off" disabled={!!busy} /></label>
              <label className="form-field">Password<input type="password" value={pass} onChange={(e) => setPass(e.target.value)} maxLength={500} autoComplete="new-password" placeholder={view.smtp.passwordSet ? 'Saved. Leave blank to keep it' : ''} disabled={!!busy} /></label>
            </div>
            <div className={styles.inlineForm}>
              <label className="form-field">From address<input type="email" value={from} onChange={(e) => setFrom(e.target.value)} placeholder="hello@yourbusiness.com" maxLength={254} required disabled={!!busy} /></label>
              <label className="form-field">From name<input value={fromName} onChange={(e) => setFromName(e.target.value)} placeholder="Dapper Detailing" maxLength={120} disabled={!!busy} /></label>
            </div>
            <p className={styles.footnote}>Port 587 (STARTTLS) is the usual choice. Port 465 also works. The connection must be encrypted.</p>
            <div className={styles.svcActions}>
              <button className="primary" disabled={!!busy || !host.trim() || !from.trim()}>{busy === 'smtp' ? 'Saving…' : 'Save email settings'}</button>
              {view.smtp.saved && <button type="button" className="secondary" disabled={!!busy} onClick={() => setConfirm('smtp')}>Remove saved settings</button>}
            </div>
            <div className={styles.inlineForm} style={{ alignItems: 'flex-end', marginTop: 12 }}>
              <label className="form-field">Send a test email to<input type="email" value={mailTo} onChange={(e) => setMailTo(e.target.value)} placeholder="you@example.com" maxLength={254} disabled={!!busy} /></label>
              <button type="button" className="secondary" disabled={!!busy || !mailTo.trim() || !view.emailWorking} onClick={testMail}>{busy === 'mail-test' ? 'Sending…' : 'Send test email'}</button>
            </div>
          </form>

          <form onSubmit={saveSms} className={styles.resource}>
            <div className={styles.rowBetween}>
              <h3 style={{ margin: 0 }}>Text messages (Twilio)</h3>
              <span className={styles.personSub}>{view.sms.saved ? 'Twilio settings saved' : 'Not set up'}</span>
            </div>
            <div className={styles.inlineForm}>
              <label className="form-field">Account SID<input value={sid} onChange={(e) => setSid(e.target.value)} placeholder="AC…" maxLength={40} autoComplete="off" required disabled={!!busy} /></label>
              <label className="form-field">Auth Token<input type="password" value={token} onChange={(e) => setToken(e.target.value)} maxLength={100} autoComplete="new-password" placeholder={view.sms.tokenSet ? 'Saved. Leave blank to keep it' : ''} required={!view.sms.tokenSet} disabled={!!busy} /></label>
            </div>
            <label className="form-field">Send from (a Twilio number or Messaging Service SID)<input value={num} onChange={(e) => setNum(e.target.value)} placeholder="+17055550100 or MG…" maxLength={40} autoComplete="off" required disabled={!!busy} /></label>
            <p className={styles.footnote}>Find these in your Twilio console. Numbers need the country code (+1 for Canada and the US). Twilio charges for every text sent. Saving here lets you test texting; customer reminders and confirmations by text are not switched on yet.</p>
            <div className={styles.svcActions}>
              <button className="primary" disabled={!!busy || !sid.trim() || !num.trim()}>{busy === 'sms' ? 'Saving…' : 'Save Twilio settings'}</button>
              {view.sms.saved && <button type="button" className="secondary" disabled={!!busy} onClick={checkSms}>{busy === 'sms-check' ? 'Checking…' : 'Check connection'}</button>}
              {view.sms.saved && <button type="button" className="secondary" disabled={!!busy} onClick={() => setConfirm('sms')}>Remove saved settings</button>}
            </div>
            {view.sms.saved && (
              <div className={styles.inlineForm} style={{ alignItems: 'flex-end', marginTop: 12 }}>
                <label className="form-field">Send a test text to<input type="tel" value={smsTo} onChange={(e) => setSmsTo(e.target.value)} placeholder="+17055550123" maxLength={30} disabled={!!busy} /></label>
                <button type="button" className="secondary" disabled={!!busy || !smsTo.trim()} onClick={testSms}>{busy === 'sms-test' ? 'Sending…' : 'Send test text'}</button>
              </div>
            )}
          </form>
        </>
      )}
      <ConfirmDialog isOpen={confirm !== null} title="Remove saved settings" entityName={confirm === 'smtp' ? 'Email settings' : 'Twilio settings'} entityType="Settings" actionType="delete" isBusy={!!busy}
        explanation={confirm === 'smtp' ? 'Email goes back to the server’s own settings, or stops if the server has none.' : 'Text messaging stops until you save Twilio settings again.'}
        onConfirm={() => void remove()} onCancel={() => setConfirm(null)} />
    </div>
  );
}
