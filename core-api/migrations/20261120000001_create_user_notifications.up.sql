-- Migration: 20261120000001_create_user_notifications.up.sql
-- Description: In-app user notifications table and performance indexes

CREATE TABLE IF NOT EXISTS user_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    tenant_id TEXT,
    event_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_notifications_user_unread 
    ON user_notifications (user_id, is_read, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_notifications_tenant 
    ON user_notifications (tenant_id, created_at DESC) 
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_notifications_created_at 
    ON user_notifications (created_at);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'swiftdrop_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON user_notifications TO swiftdrop_app;
    END IF;
END $$;
