# Dapper Detailing

Canonical business decisions and implementation evidence live in Dapper Detailing Knowledge. This file summarizes product context for design tools; it does not approve policy or supersede the vault.

## Platform and stack
Existing Next.js/React web, unified Go/PostgreSQL API; native Windows development, no WSL. Android/iOS companion capabilities are required for the requested physical Square Reader and ongoing on-shift location tracking; platform/provider integration remains separately verified work.

## Audience and purpose
Employee Workspace: dealership-assigned staff clock in/out, share next-week availability, book walk-ins with validated contact formats, view prebooked customers in a full month/week/day calendar, accept authorized cash/Reader payments and generate invoices. Admin assigns worksite; employee cannot select scope. Master provides central administration; public website and customer account are separate surfaces on the same authority.

## Confirmed constraints
Dapper identity and red accent; real authenticated scoped persistence; no demo operations or fabricated provider results. Walk-ins require valid email and international mobile-number formats only. No OTP is required; contact ownership is not asserted. Tracking only during clocked-in shifts. Android phones/tablets and iPhone/iPad required. Privacy/device/provider details remain pending per employee brief.

## Success
Staff find their next action immediately on a phone, see assignment and work/payment/tracking state accurately, and complete tasks without navigating administrative forms. Missing prerequisites are explained. Historical commercial data and authorization boundaries survive UI changes. No global bypass or automatic grants.
