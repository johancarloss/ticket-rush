#!/bin/sh
# One schema and one user per service, each user restricted to its own schema (ADR-005).
# Keycloak gets a separate database that it manages itself (ADR-007).
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE CONNECT ON DATABASE ticketrush FROM PUBLIC;

CREATE USER inventory_svc WITH PASSWORD '$INVENTORY_DB_PASSWORD';
CREATE USER checkout_svc WITH PASSWORD '$CHECKOUT_DB_PASSWORD';
CREATE USER notification_svc WITH PASSWORD '$NOTIFICATION_DB_PASSWORD';
CREATE USER stats_svc WITH PASSWORD '$STATS_DB_PASSWORD';

CREATE SCHEMA inventory AUTHORIZATION inventory_svc;
CREATE SCHEMA checkout AUTHORIZATION checkout_svc;
CREATE SCHEMA notification AUTHORIZATION notification_svc;
CREATE SCHEMA stats AUTHORIZATION stats_svc;

GRANT CONNECT ON DATABASE ticketrush TO inventory_svc, checkout_svc, notification_svc, stats_svc;

ALTER ROLE inventory_svc SET search_path = inventory;
ALTER ROLE checkout_svc SET search_path = checkout;
ALTER ROLE notification_svc SET search_path = notification;
ALTER ROLE stats_svc SET search_path = stats;

CREATE USER keycloak_svc WITH PASSWORD '$KEYCLOAK_DB_PASSWORD';
CREATE DATABASE keycloak OWNER keycloak_svc;
SQL
