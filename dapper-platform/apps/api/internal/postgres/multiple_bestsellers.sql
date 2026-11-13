-- error404 #103 (2026-11-14): several bestsellers per category (one per size or vehicle tab), so the one-star rule goes.
DROP INDEX IF EXISTS service_publications_one_star;
