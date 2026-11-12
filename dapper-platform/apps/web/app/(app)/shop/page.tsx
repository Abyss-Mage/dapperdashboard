import { redirect } from 'next/navigation';

// The shop iPad now uses the workspace itself: sign in at /workspace with the shop login and its clock page is the name list (owner, 2026-11-12).
export default function OldShopPage() {
  redirect('/workspace');
}
