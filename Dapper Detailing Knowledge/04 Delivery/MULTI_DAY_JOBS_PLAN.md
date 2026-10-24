# Plan: scheduling the 15 jobs longer than one opening day (2026-10-24)
Status: PLAN ONLY. Nothing built. Needs the owner's answers in section 5.
Jobs (durations from the legacy BookingPress export): Aircraft Medium Ceramic Coating 15h, Aircraft Medium Paint Correction 18h, Aircraft Large Signature Full Detail 18h, Aircraft Large Ceramic Coating 24h, Aircraft Large Paint Correction 24h, and all 10 PPF services (Front = 1 day, Full = 2 days; Hatchback, Sedan, SUV, Pickup, VAN).

## 1. Why they were not created
The availability engine (internal/scheduling) treats a booking as one continuous block from start (minus buffer) to end (plus buffer). That block must lie inside the resource's opening windows; windows that touch are merged, but a closed gap (the 21:00 to 08:00 night) can never be crossed. Main Office is open 08:00-21:00 (780 minutes), so anything longer than 780 minutes can never be placed. A reservation is a single interval, and customer slots come from the published booking offer (lead minutes, advance days, slot step) anchored to the start of the local day. There is no setting for "the job continues tomorrow".

## 2. Options
A. Extended resource (no code, today). Add a third resource, for example "Main Office extended jobs", 1 at a time, open 00:00-24:00 every day (adjacent windows merge), and create the 15 rules with the exported durations (15h, 18h, 24h, 48h). A job blocks only that resource, so normal bays keep working.
   + Staff can book them immediately (walk-in and internal bookings take a typed start time).
   - The resource looks open at night, so a customer-facing slot list would offer overnight starts; the job is modelled as continuous clock time (a 48h PPF holds the resource for two full days including nights and Sundays), not as working days.
   - Safe only while these are staff-booked or published as quote-required (no self-service slots).
B. Working-time rules (new engine feature, recommended long-term). A rule gets "working time": the duration counts only opening hours and may continue across closed hours/nights. Start Mon 08:00 on a 24h-of-work job -> finishes after the second day's opening hours; the reservation occupies the resource from the start until the last working minute, skipping closed time. Needs: a flag on service_scheduling_rules (migration), a new calculation in scheduling.Evaluate/AvailableStarts, reservation/reschedule/offer snapshots carrying it, tests (DST, closures/holidays, capacity 1). Medium-sized change; no UI beyond a checkbox "continues across closed hours" in the rule form.
C. Split into stages. One rule per day of work ("PPF Full - day 1", "day 2") booked as two linked bookings. No engine change, but two bookings per job, customers see two appointments, and day 2 can fail to find space. Not recommended.
D. Quote-required only. Keep the 15 out of online booking: publish them as quote-required offers and schedule by hand after the quote. Still needs a rule to hold capacity, so it pairs with A or B.

## 3. Recommendation
Now: A, limited to staff-booked and quote-required use, so these services stop being invisible. Next: B if the shop wants customers to book them online or wants nights and closed days excluded from the clock. Do not publish self-service slots for the extended resource until B (or an approved start-time rule) exists.

## 4. Build steps for A (about 10 minutes, existing tools)
1. Create resource "Main Office extended jobs" (1 at a time) with weekly hours 00:00-24:00 Mon-Sun, America/Toronto.
2. Create 15 rules, duration as exported, buffers 0, units 1 (use scripts/seed-scheduling with a -long flag).
3. Show them in Walk-in with the longer durations; staff pick start times.
4. Verify availability for one 24h and one 48h job against existing bookings; confirm they do not block the normal bays.
Build steps for B: migration (rule flag), engine change + tests, rule form checkbox, then move the 15 rules onto the normal resources with the flag on.

## 5. Questions for the owner
1. Are these durations continuous clock time (a 24h job runs overnight) or working time (the crew works the opening hours each day until done)? Legacy "1 d / 2 d" for PPF may mean 1 or 2 working days.
2. If working time: how many working hours is "1 day" for PPF (full 13h opening day, or a shorter shift)?
3. May two such jobs overlap (two crews), or strictly one at a time? Legacy export says 1 at a time.
4. Should customers be able to book these online, or quote-first with staff scheduling?
5. Approve option A now, option B later, or B directly?
