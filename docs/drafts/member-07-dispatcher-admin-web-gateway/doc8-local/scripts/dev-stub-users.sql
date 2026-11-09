-- LOCAL DEMO STUB ONLY. Not a migration (Member 9 D4: stubs never go in core-api/migrations/).
-- The real users table is Member 1's 000001. local-up.sh runs this before the migrations,
-- so golang-migrate can start at 000002 as in the real repo.
CREATE TABLE IF NOT EXISTS users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      varchar(255) NOT NULL UNIQUE,
    role       varchar(20)  NOT NULL CHECK (role IN ('ADMIN','DISPATCHER','COURIER','MERCHANT','CUSTOMER')),
    created_at timestamptz  NOT NULL DEFAULT now()
);
