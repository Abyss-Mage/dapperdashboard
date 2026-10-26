-- Weekly opening hours for a scheduling resource. The weekly template is the source; dated opening windows in rules.Open are
-- generated from it (12 weeks ahead, rolled forward by the API) so availability keeps working without hand-entered dates.
ALTER TABLE scheduling_resources ADD COLUMN weekly_hours jsonb CHECK(weekly_hours IS NULL OR jsonb_typeof(weekly_hours)='array');
