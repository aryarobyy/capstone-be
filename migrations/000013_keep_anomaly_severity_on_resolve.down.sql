UPDATE anomalies SET status = 'NORMAL' WHERE status = 'WARNING' AND resolved_at IS NOT NULL;
