#!/bin/bash
set -e

# Create database and user.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  SELECT 'CREATE DATABASE airalert'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'airalert')\gexec
  DO \$\$
    BEGIN
      CREATE USER airalert WITH PASSWORD '$AIR_ALERT_PGPASS';
      EXCEPTION WHEN DUPLICATE_OBJECT THEN
      RAISE NOTICE 'skipping user creation';
    END
  \$\$;
EOSQL

# Now initialize tables.
PGPASSWORD="$AIR_ALERT_PGPASS" psql -v ON_ERROR_STOP=1 --username airalert --dbname airalert <<-EOSQL
  CREATE TABLE IF NOT EXISTS users (
    id SERIAL NOT NULL PRIMARY KEY,
    push_url TEXT NOT NULL,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    last_crossover TIMESTAMP WITH TIME ZONE
  );
EOSQL