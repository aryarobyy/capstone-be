-- Fails (and rolls back) if two accounts differ only by email case; merge them manually first.
UPDATE users SET email = lower(trim(email)) WHERE email <> lower(trim(email));
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users (lower(email));
