-- Test-only stand-in for Member 1's users table and Member 9's worker role.
CREATE TABLE IF NOT EXISTS users (id uuid PRIMARY KEY);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_worker') THEN
    CREATE ROLE swiftdrop_worker NOLOGIN;
  END IF;
END $$;
