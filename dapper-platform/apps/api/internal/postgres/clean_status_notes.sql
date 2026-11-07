-- Status changes used to append "Operator status update" or "Employee status update" to a booking's own notes every time. They belong in the
-- booking history only; remove the leftovers and keep what people actually wrote.
UPDATE bookings SET notes = btrim(regexp_replace(regexp_replace(regexp_replace(notes, '(\s*\|\s*)?(Operator|Employee) status update', '', 'g'), '^\s*\|\s*', ''), '\s*\|\s*$', ''))
WHERE notes ~ '(Operator|Employee) status update';
