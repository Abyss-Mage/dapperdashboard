-- A deleted quotation leaves every list but keeps its audit trail, which the database does not allow to be removed.
ALTER TABLE scoped_quotations DROP CONSTRAINT scoped_quotations_status_check;
ALTER TABLE scoped_quotations ADD CONSTRAINT scoped_quotations_status_check CHECK(status IN ('draft','issued','accepted','declined','expired','superseded','fulfilled','deleted'));
ALTER TABLE scoped_quotation_events DROP CONSTRAINT scoped_quotation_events_action_check;
ALTER TABLE scoped_quotation_events ADD CONSTRAINT scoped_quotation_events_action_check CHECK(action IN ('created','edited','issued','accepted','declined','expired','superseded','fulfilled','deleted'));
