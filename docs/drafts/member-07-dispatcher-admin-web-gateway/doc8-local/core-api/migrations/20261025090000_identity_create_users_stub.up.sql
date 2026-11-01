-- DEMO STUB ONLY. The real users table is Member 1's migration.
-- It exists here so the admin tables' foreign keys work in the local demo.
CREATE TABLE IF NOT EXISTS users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      varchar(255) NOT NULL UNIQUE,
    role       varchar(20)  NOT NULL CHECK (role IN ('ADMIN','DISPATCHER','COURIER','MERCHANT','CUSTOMER')),
    created_at timestamptz  NOT NULL DEFAULT now()
);
