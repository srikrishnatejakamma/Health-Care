#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
CREATE ROLE healthcare_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
CREATE USER healthcare_app WITH PASSWORD 'local-app-password' NOSUPERUSER NOBYPASSRLS;
GRANT healthcare_runtime TO healthcare_app;
SQL
