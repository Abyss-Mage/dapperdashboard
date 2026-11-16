-- error404 #133 (owner 2026-11-15): a sale price on a service, with an optional last day (Toronto). While it runs it is the price
-- everywhere: the website, website and dashboard bookings, walk-in invoices. A franchise's copy of a central service follows the
-- central service's sale; a franchise's own service has its own. The regular price is kept, so the sale ending brings it back.
ALTER TABLE central_catalog_services ADD COLUMN sale_price_cents bigint CHECK (sale_price_cents IS NULL OR sale_price_cents >= 0);
ALTER TABLE central_catalog_services ADD COLUMN sale_ends_on date;
ALTER TABLE catalog_services ADD COLUMN sale_price_cents bigint CHECK (sale_price_cents IS NULL OR sale_price_cents >= 0);
ALTER TABLE catalog_services ADD COLUMN sale_ends_on date;

CREATE FUNCTION sale_or_price(price bigint, sale bigint, ends date) RETURNS bigint LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN sale IS NOT NULL AND (ends IS NULL OR ends >= (now() AT TIME ZONE 'America/Toronto')::date) THEN sale ELSE price END
$$;

-- The price of a franchise service right now.
CREATE FUNCTION service_effective_price(service uuid) RETURNS bigint LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN c.central_service_id IS NOT NULL AND cc.id IS NOT NULL THEN sale_or_price(c.price_cents, cc.sale_price_cents, cc.sale_ends_on)
  ELSE sale_or_price(c.price_cents, c.sale_price_cents, c.sale_ends_on) END
 FROM catalog_services c LEFT JOIN central_catalog_services cc ON cc.id=c.central_service_id WHERE c.id=service
$$;
