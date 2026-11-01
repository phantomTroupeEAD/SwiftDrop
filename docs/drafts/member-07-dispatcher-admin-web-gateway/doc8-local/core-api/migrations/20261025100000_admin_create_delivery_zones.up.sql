CREATE TABLE delivery_zones (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code                 varchar(40)  NOT NULL UNIQUE CHECK (code ~ '^ZONE_[A-Z0-9_]{2,35}$'),
    name                 varchar(100) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
    description          text,
    base_surcharge_cents bigint  NOT NULL DEFAULT 0 CHECK (base_surcharge_cents >= 0),
    currency             char(3) NOT NULL DEFAULT 'LKR' CHECK (currency = 'LKR'),
    enabled              boolean NOT NULL DEFAULT true,
    boundary_polygon     jsonb,
    version              integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by           uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by           uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
