-- Anomaly records are append-only and keep their severity. The evaluator only ever raises WARNING.
UPDATE anomalies SET status = 'WARNING' WHERE status = 'NORMAL' AND resolved_at IS NOT NULL;
