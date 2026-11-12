// Shared bits of the dealership portal (error404 #26–#32).

export const API = '/api/v1/dealer-portal';

export type Summary = { id: string; invoiceNumber: string; poNumber: string; status: string; overdue: boolean; invoiceDate?: string; dueDate?: string; issuedAt?: string; dueAt?: string; createdAt: string; totalCents: number; dueCents: number };
export type Line = { description: string; detail: string; quantity: number; unitPriceCents: number; vin: string; stockNumber: string };
export type Invoice = Summary & { subtotalCents: number; discountCents: number; discountPercent: number; taxCents: number; paidCents: number; notes: string; lines: Line[]; dealershipName: string };
export type Card = { id: string; brand: string; last4: string; expMonth: number; expYear: number; isDefault: boolean; createdAt: string };
export type Payment = { id: string; invoiceId: string; invoiceNumber: string; amountCents: number; method: string; status: string; recordedAt: string };

export const money = (c: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
/** 21/Nov/2025, the date style of the owner's invoices. */
export function day(iso?: string) {
  if (!iso) return '—';
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  const d = m && iso.length === 10 ? new Date(+m[1], +m[2] - 1, +m[3]) : new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return `${String(d.getDate()).padStart(2, '0')}/${MONTHS[d.getMonth()]}/${d.getFullYear()}`;
}
export const invoiceDay = (i: Summary) => i.invoiceDate ?? i.issuedAt ?? i.createdAt;
export const dueDay = (i: Summary) => i.dueDate ?? i.dueAt;

export function stateOf(i: { status: string; overdue: boolean }): { label: string; tone: 'paid' | 'unpaid' | 'late' | 'void' } {
  if (i.status === 'paid') return { label: 'Paid', tone: 'paid' };
  if (i.status === 'void') return { label: 'Void', tone: 'void' };
  if (i.overdue) return { label: 'Past due', tone: 'late' };
  return { label: i.status === 'partially_paid' ? 'Part paid' : 'Unpaid', tone: 'unpaid' };
}

export const METHOD: Record<string, string> = { square_card: 'Card', square_terminal: 'Card', cash: 'Cash', bank_transfer: 'Bank transfer', cheque: 'Cheque' };

/** Reads a JSON reply; a signed-out portal goes to the sign-in page. */
export async function readJson<T>(r: Response): Promise<T> {
  const body = await r.json().catch(() => ({}));
  if (r.status === 401 && body.code === 'dealer_signed_out') {
    window.location.assign(`/dealer/sign-in?next=${encodeURIComponent(window.location.pathname + window.location.search)}`);
    throw new Error('Sign in to continue.');
  }
  if (!r.ok) throw new Error(body.error ?? 'Something went wrong. Try again.');
  return body as T;
}
export const post = (body?: unknown, method = 'POST'): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
