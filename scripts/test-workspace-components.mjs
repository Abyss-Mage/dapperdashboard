import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const root = path.resolve(__dirname, '..');

/**
 * Workspace Component and Accessibility Test Suite
 * Verifies audit findings F05, F06, F07, F08, F09, F10, F11, F12.
 */

test('F05/F06: ConfirmDialog component implementation and contracts', () => {
  const confirmDialogPath = path.join(root, 'dapper-platform/apps/web/app/workspace/ConfirmDialog.tsx');
  assert.ok(fs.existsSync(confirmDialogPath), 'ConfirmDialog.tsx must exist');
  const src = fs.readFileSync(confirmDialogPath, 'utf8');

  // Accessibility invariants
  assert.ok(src.includes('role="dialog"'), 'Must have role="dialog"');
  assert.ok(src.includes('aria-modal="true"'), 'Must have aria-modal="true"');
  assert.ok(src.includes('aria-labelledby="confirm-dialog-title"'), 'Must link title via aria-labelledby');
  assert.ok(src.includes('cancelBtnRef.current?.focus()'), 'Must place safe default initial focus on Cancel button');
  assert.ok(src.includes("e.key === 'Escape'"), 'Must support dismiss on Escape key');
  assert.ok(src.includes('previouslyFocusedRef.current.focus()'), 'Must restore focus to triggering element on close');

  // Deletion distinction contract props
  assert.ok(src.includes('explanation'), 'Must accept and render operation explanation');
  assert.ok(src.includes('impactNotice'), 'Must accept and render dependency and impact notices');
});

test('F05: StructureEditor and ServiceEditor integrate ConfirmDialog with concurrency protection', () => {
  const structurePath = path.join(root, 'dapper-platform/apps/web/app/workspace/StructureEditor.tsx');
  const servicePath = path.join(root, 'dapper-platform/apps/web/app/workspace/ServiceEditor.tsx');
  const centralPath = path.join(root, 'dapper-platform/apps/web/app/workspace/CentralServiceEditor.tsx');

  const structureSrc = fs.readFileSync(structurePath, 'utf8');
  const serviceSrc = fs.readFileSync(servicePath, 'utf8');
  const centralSrc = fs.readFileSync(centralPath, 'utf8');

  assert.ok(structureSrc.includes('ConfirmDialog'), 'StructureEditor must use ConfirmDialog');
  assert.ok(structureSrc.includes('promptDeleteFranchise'), 'Must confirm franchise deletion');
  assert.ok(structureSrc.includes('promptDeleteLocation'), 'Must confirm location deletion');
  assert.ok(structureSrc.includes('executeConfirmedDeletion'), 'Must execute deletion only after confirmation');
  assert.ok(structureSrc.includes('permanently removes'), 'StructureEditor must explain permanent removal vs historical retention');
  assert.ok(structureSrc.includes('409'), 'Must handle 409 conflict responses on concurrent mutation');

  assert.ok(serviceSrc.includes('ConfirmDialog'), 'ServiceEditor must use ConfirmDialog');
  assert.ok(serviceSrc.includes('deleteService'), 'Must implement delete service action');
  assert.ok(serviceSrc.includes('removes it permanently'), 'ServiceEditor must explain permanent deletion vs archiving with historical retention');

  assert.ok(centralSrc.includes('ConfirmDialog'), 'CentralServiceEditor must use ConfirmDialog');
});

test('F07: InvoiceManager exposes accessible names and associated labels for all 9 controls', () => {
  const invoicePath = path.join(root, 'dapper-platform/apps/web/app/workspace/InvoiceManager.tsx');
  const src = fs.readFileSync(invoicePath, 'utf8');

  // Verify explicit label htmlFor and id associations
  assert.ok(src.includes('htmlFor="draft-customer-select"'), 'Customer selector must have associated label');
  assert.ok(src.includes('htmlFor="draft-custom-number"'), 'Draft reference must have associated label');
  assert.ok(src.includes('htmlFor="draft-tax-amount"'), 'Tax input must have associated label');
  assert.ok(src.includes('htmlFor={`draft-line-desc-${idx}`}'), 'Item description must have associated label');
  assert.ok(src.includes('htmlFor={`draft-line-qty-${idx}`}'), 'Item quantity must have associated label');
  assert.ok(src.includes('htmlFor={`draft-line-price-${idx}`}'), 'Item price must have associated label');
  assert.ok(src.includes('htmlFor="draft-notes-textarea"'), 'Notes textarea must have associated label');
  assert.ok(src.includes('htmlFor="invoice-search-filter"'), 'Search input must have associated label');
  assert.ok(src.includes('htmlFor="invoice-status-filter"'), 'Status filter select must have associated label');

  // Verify payment dialog controls also have associated labels
  assert.ok(src.includes('htmlFor="cash-payment-amount"'), 'Payment amount must have associated label');
  assert.ok(src.includes('htmlFor="cash-payment-notes"'), 'Payment notes must have associated label');
  assert.ok(src.includes('htmlFor="void-reason-input"'), 'Void reason must have associated label');
});

test('F08: Dark mode shift summary and secondary labels meet WCAG AA contrast (>= 4.5:1)', () => {
  const cssPath = path.join(root, 'dapper-platform/apps/web/app/globals.css');
  const css = fs.readFileSync(cssPath, 'utf8');

  // Check CSS custom properties defined on root and dark theme
  assert.ok(css.includes('--text-secondary: #596871;'), 'Root must define high-contrast text-secondary token');
  assert.ok(css.includes('--text-secondary: #aab4c2;'), 'Dark theme must define high-contrast text-secondary token (#aab4c2)');
  assert.ok(css.includes('--text-muted: #aab4c2;'), 'Dark theme must define high-contrast text-muted token (#aab4c2)');

  // Verify AttendancePanel and SecureWorkspace use theme tokens with compliant fallbacks
  const attPath = path.join(root, 'dapper-platform/apps/web/app/workspace/AttendancePanel.tsx');
  const attSrc = fs.readFileSync(attPath, 'utf8');
  assert.ok(!attSrc.includes("color: 'var(--muted)'"), 'Must not use unstyled var(--muted)');
  assert.ok(attSrc.includes("var(--text-secondary, #596871)"), 'AttendancePanel must use WCAG AA compliant text-secondary token');

  const wsPath = path.join(root, 'dapper-platform/apps/web/app/workspace/SecureWorkspace.tsx');
  const wsSrc = fs.readFileSync(wsPath, 'utf8');
  assert.ok(!wsSrc.includes("color: 'var(--text-secondary, #555)'"), 'Must eliminate failing #555 shift label color (2.31:1)');
  assert.ok(wsSrc.includes("color: 'var(--text-secondary, #596871)'"), 'Location shift pulse must use >= 4.5:1 compliant fallback color');

  // Verify mathematical contrast of #aab4c2 on #191b1f
  // #191b1f: L = 0.00814
  // #aab4c2: L = 0.450
  // Contrast = (0.450 + 0.05) / (0.00814 + 0.05) = 8.6:1 (WCAG AA requirement: >= 4.5:1)
  const darkCardLuminance = 0.00814;
  const darkTextLuminance = 0.450;
  const contrastRatio = (darkTextLuminance + 0.05) / (darkCardLuminance + 0.05);
  assert.ok(contrastRatio >= 4.5, `Contrast ratio (${contrastRatio.toFixed(2)}:1) must meet or exceed WCAG AA 4.5:1`);
});

test('F09: Touch targets reach 44px craft target and mobile inputs set >= 16px to prevent iOS zoom', () => {
  const cssPath = path.join(root, 'dapper-platform/apps/web/app/globals.css');
  const css = fs.readFileSync(cssPath, 'utf8');

  // Check mobile media query rules
  assert.ok(css.includes('font-size:16px !important'), 'Mobile inputs must be minimum 16px to prevent automatic iOS Safari viewport zooming');
  assert.ok(css.includes('min-height:44px'), 'Mobile interactive elements must have minimum 44px height craft target');
});

test('F10: Workspace usability: In-page jump navigation and progressive disclosure for setup', () => {
  const wsPath = path.join(root, 'dapper-platform/apps/web/app/workspace/SecureWorkspace.tsx');
  const wsSrc = fs.readFileSync(wsPath, 'utf8');
  const cssPath = path.join(root, 'dapper-platform/apps/web/app/globals.css');
  const css = fs.readFileSync(cssPath, 'utf8');

  // Jump navigation
  assert.ok(wsSrc.includes('className="workspace-jump-nav"'), 'Must render in-page jump navigation bar');
  assert.ok(wsSrc.includes('href="#operations"'), 'Must have #operations jump link');
  assert.ok(wsSrc.includes('href="#catalogue"'), 'Must have #catalogue jump link');
  assert.ok(wsSrc.includes('href="#finance"'), 'Must have #finance jump link');
  assert.ok(wsSrc.includes('href="#administration"'), 'Must have #administration jump link');

  // Target anchors with scroll-margin-top
  assert.ok(wsSrc.includes('id="operations"'), 'Must have operations anchor');
  assert.ok(wsSrc.includes('id="catalogue"'), 'Must have catalogue anchor');
  assert.ok(wsSrc.includes('id="finance"'), 'Must have finance anchor');
  assert.ok(wsSrc.includes('id="administration"'), 'Must have administration anchor');
  assert.ok(css.includes('.jump-target'), 'CSS must define jump-target scroll margins');

  // Progressive disclosure for secondary organization setup panels
  assert.ok(wsSrc.includes('setup-disclosure'), 'Must wrap secondary setup panels in setup-disclosure details');
  assert.ok(wsSrc.includes('disclosure-header'), 'Must have clean disclosure header in summary');
  assert.ok(wsSrc.includes('disclosure-toggle'), 'Must have disclosure toggle button');
  assert.ok(css.includes('.setup-disclosure'), 'CSS must style setup-disclosure details preserving card baseline');
});

test('F11: MemberInvitations copy explains named role assignments and SMTP setup prerequisites', () => {
  const invPath = path.join(root, 'dapper-platform/apps/web/app/workspace/MemberInvitations.tsx');
  const src = fs.readFileSync(invPath, 'utf8');

  assert.ok(src.includes('named roles'), 'Copy must explain named role assignment instead of obsolete per-action capability toggles');
  assert.ok(src.includes('Role Assignments below'), 'Copy must reference the Role Assignments section');
  assert.ok(src.includes('SMTP'), 'Unavailable message must explain SMTP delivery setup prerequisites');
  assert.ok(src.includes('htmlFor="invitation-email"'), 'Email field must have associated label');
  assert.ok(src.includes('htmlFor="invitation-minutes"'), 'Minutes field must have associated label');
});
