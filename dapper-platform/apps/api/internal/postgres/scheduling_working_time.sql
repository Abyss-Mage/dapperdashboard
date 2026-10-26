-- A rule may count its duration as working time: the job continues across closed hours (nights, closure days) until the duration of
-- opening hours has been spent. Needed for jobs longer than one opening day (multi-day PPF, aircraft ceramic and paint correction).
ALTER TABLE service_scheduling_rules ADD COLUMN working_time boolean NOT NULL DEFAULT false;
