-- Fails if Member 8's webhook_deliveries still references rows; roll back M8 first.
DROP TABLE IF EXISTS webhook_endpoints;
