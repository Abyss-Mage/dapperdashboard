import type { ReactElement } from 'react';

export type OwnerIconName =
  | 'overview'
  | 'calendar'
  | 'customers'
  | 'work'
  | 'finance'
  | 'dealership'
  | 'marketing'
  | 'admin'
  | 'menu'
  | 'close'
  | 'pin'
  | 'refresh'
  | 'out'
  | 'arrow';

const paths: Record<OwnerIconName, ReactElement> = {
  overview: <><rect x="3.5" y="3.5" width="7" height="8" rx="2" /><rect x="13.5" y="3.5" width="7" height="5" rx="2" /><rect x="13.5" y="11.5" width="7" height="9" rx="2" /><rect x="3.5" y="14.5" width="7" height="6" rx="2" /></>,
  calendar: <><rect x="3.5" y="5" width="17" height="15.5" rx="3" /><path d="M3.5 10h17M8 3v4M16 3v4" /></>,
  customers: <><circle cx="9" cy="8.5" r="3.2" /><path d="M3.5 19.5c.4-3.3 2.7-5.2 5.5-5.2s5.1 1.9 5.5 5.2" /><path d="M16 5.6a3 3 0 0 1 0 5.8M17.5 14.6c1.8.6 3 2.3 3.2 4.9" /></>,
  work: <><path d="M14.5 6.5a4 4 0 0 0-5 5L4 17l3 3 5.5-5.5a4 4 0 0 0 5-5l-2.5 2.5-2.5-.5-.5-2.5z" /></>,
  finance: <><rect x="3" y="5.5" width="18" height="13" rx="3" /><path d="M3 10h18M7 15h3" /></>,
  dealership: <><path d="M4 15.5 6 9.5a2 2 0 0 1 1.9-1.4h8.2A2 2 0 0 1 18 9.5l2 6" /><rect x="3" y="15.5" width="18" height="4" rx="1.8" /><circle cx="7.5" cy="17.5" r=".6" /><circle cx="16.5" cy="17.5" r=".6" /></>,
  marketing: <><path d="M4 13.5v-3l11-5v13l-11-5z" /><path d="M7 14.5l1.2 4.5h2.6l-1-4M18 9.5a3.5 3.5 0 0 1 0 5" /></>,
  admin: <><circle cx="12" cy="12" r="3" /><path d="M12 3.5v2.2M12 18.3v2.2M3.5 12h2.2M18.3 12h2.2M6 6l1.6 1.6M16.4 16.4 18 18M18 6l-1.6 1.6M7.6 16.4 6 18" /></>,
  menu: <path d="M4 7h16M4 12h16M4 17h16" />,
  close: <path d="m6 6 12 12M18 6 6 18" />,
  pin: <><path d="M12 21s6.5-5.6 6.5-11a6.5 6.5 0 1 0-13 0c0 5.4 6.5 11 6.5 11z" /><circle cx="12" cy="10" r="2.3" /></>,
  refresh: <><path d="M20 11a8 8 0 0 0-14-4.5L4 9" /><path d="M4 4v5h5M4 13a8 8 0 0 0 14 4.5L20 15" /><path d="M20 20v-5h-5" /></>,
  out: <><path d="M10 4.5H7a2.5 2.5 0 0 0-2.5 2.5v10A2.5 2.5 0 0 0 7 19.5h3M15 8l4 4-4 4M19 12H9.5" /></>,
  arrow: <path d="M9 6l6 6-6 6" />,
};

export function OwnerIcon({ name, size = 20 }: { name: OwnerIconName; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name]}
    </svg>
  );
}
