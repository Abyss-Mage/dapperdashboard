import { redirect } from 'next/navigation';

// The old open shop clock link: the shop iPad now signs in with the franchise's shop login at /shop (owner decision 2026-11-11, error404 #20).
export default function OldShopClock() {
  redirect('/workspace');
}
