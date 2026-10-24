# Employee Workspace interface

Scope: `/workspace` only. Canonical authority: employee brief and direct user changes on 2026-10-24. Master remains `/owner`; public website is a separate surface.

The employee surface has four destinations: Today, Bookings, Walk-in and My shifts. Assignment is read-only. The desktop agenda and shift rail become a shift-first column with safe-area bottom navigation on phones. Administration, global metrics, franchise/location selection and role editing are absent from this surface. Shared Master components are retained.

Manrope typography, Dapper red #bc2532, warm #f5f4f1 canvas, white surfaces, ink #202321 and muted #606761 define the light theme. The CSS module contains dark tokens and scoped controls. Inputs use 16px text; principal actions are 46–48px high. Lucide icons supplement text. Keyboard focus, skip link, error/status announcements, loading/empty/disabled states and touch hover gating are explicit. No decorative raster assets are shipped.

Bookings defaults to a complete six-week month grid, with week/day agenda views, period navigation, date picker, appointment details and invoice context. Actual selected date ranges load from a dealership-scoped API, capped at 500 with an explicit narrowing message. On phones, the month grid scrolls within its own region, not the page. Times use the assigned worksite timezone; durations are not inferred from a card height. Drag-to-reschedule is not implemented or implied.

Walk-in requires name, valid email and international mobile number including country code. Spacing/brackets/dashes normalize to E.164-shaped syntax. No OTP or contact-ownership badge exists. Staff choose only configured exact service/add-on/resource combinations; a calendar date/time is rechecked against opening hours, buffers and remaining capacity in the booking transaction. Success means pending booking plus reserved capacity, not payment or an approved confirmation transition.

GSAP 3.15.0 with @gsap/react 2.1.2 uses scoped useGSAP hooks, revertOnUpdate and matchMedia cleanup. Headings move 6px over 180ms; success feedback moves 4px over 160ms with power3.out. Content remains visible and controls remain usable throughout. Reduced-motion disables these tweens. No entry stagger delays clock or payment actions.

Evidence: output/acceptance/employee-workspace-desktop.png, employee-workspace-mobile.png, employee-calendar-desktop.png, employee-calendar-mobile.png and employee-invoice-paid.png. These show explicitly synthetic isolated records, never application database records. Actual browser persistence verifies attendance, availability, cash receipt and no-OTP walk-in capacity/dealership linkage. Print rules are source-present; actual printer/PDF output is not certified. Dark tokens are source-present; physical Android/iOS and comprehensive accessibility testing remain open.

Finish verdict for the complete six-workflow brief: fix. Ongoing background tracking, physical Square Reader and automatic invoice issuance need separate integration/configuration/device acceptance. This verdict does not block local review of the implemented employee web increment and does not claim full production readiness.


Update2026-10-24: accessible compact sun/moon toggle is in the masthead beside sign-out and shares the existing persisted theme setting. Authenticated actual Main Office dark/light and390px phone layouts checked; dark persists through reload. This is a focused theme verification, not comprehensive dark-theme/physical-device/accessibility certification.
