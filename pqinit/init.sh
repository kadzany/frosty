#!/bin/bash
set -e

# Create databases
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE test_database;
    GRANT ALL PRIVILEGES ON DATABASE def_database TO postgres;
    GRANT ALL PRIVILEGES ON DATABASE test_database TO postgres;
EOSQL