-- Employees are assigned to a franchise site (the location row). A partner dealership is optional and separate.
-- Previously the assignment required a dealership account, which forced internal worksites (e.g. Main Office) to be
-- modelled as fake dealership accounts. This migration makes the dealership optional everywhere it was mandatory and
-- re-states location integrity explicitly (it used to come implicitly from the dealership foreign key).
ALTER TABLE employee_worksite_assignments ALTER COLUMN dealership_id DROP NOT NULL;
ALTER TABLE employee_worksite_assignments ADD CONSTRAINT employee_worksite_assignments_location_fk FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id);

ALTER TABLE employee_availability_submissions ALTER COLUMN dealership_id DROP NOT NULL;
ALTER TABLE employee_availability_submissions ADD CONSTRAINT employee_availability_submissions_location_fk FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id);
DO $$
DECLARE c text;
BEGIN
 SELECT conname INTO c FROM pg_constraint WHERE conrelid='employee_availability_submissions'::regclass AND contype='u' AND pg_get_constraintdef(oid) LIKE '%dealership_id%';
 IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE employee_availability_submissions DROP CONSTRAINT %I', c); END IF;
END $$;
-- Availability is one submission per person per week at a site, whatever dealership they happen to be focused on.
ALTER TABLE employee_availability_submissions ADD CONSTRAINT employee_availability_one_per_week UNIQUE(organization_id,location_id,principal_id,week_start);

-- A booking link with no dealership means "visible to employees of this site"; with a dealership it is limited to
-- employees assigned to that dealership.
ALTER TABLE employee_booking_worksites ALTER COLUMN dealership_id DROP NOT NULL;
ALTER TABLE employee_booking_worksites ADD CONSTRAINT employee_booking_worksites_location_fk FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id);

-- Customer-car posts take their dealership (if any) from the booking link; only key-tag posts require one.
ALTER TABLE employee_upload_posts ALTER COLUMN dealership_id DROP NOT NULL;
ALTER TABLE employee_upload_posts ADD CONSTRAINT employee_upload_posts_tag_dealership CHECK(kind<>'dealership_tag' OR dealership_id IS NOT NULL);

-- Normalise existing data: an internal worksite account (anything that is not a partner dealership) is not a dealership.
UPDATE employee_worksite_assignments w SET dealership_id=NULL,version=w.version+1 FROM dealership_accounts d
 WHERE d.id=w.dealership_id AND d.organization_id=w.organization_id AND d.location_id=w.location_id AND d.account_type<>'dealership';
UPDATE employee_availability_submissions s SET dealership_id=NULL FROM dealership_accounts d
 WHERE d.id=s.dealership_id AND d.organization_id=s.organization_id AND d.location_id=s.location_id AND d.account_type<>'dealership';
UPDATE employee_booking_worksites w SET dealership_id=NULL FROM dealership_accounts d
 WHERE d.id=w.dealership_id AND d.organization_id=w.organization_id AND d.location_id=w.location_id AND d.account_type<>'dealership';
