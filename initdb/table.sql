CREATE DATABASE akara;
\c akara;

CREATE TABLE chip_data (
    chipid jsonb PRIMARY KEY,
    token text,
    time timestamp with time zone
);

CREATE TABLE weather (
    chipid jsonb,
    humidity double precision,
    temperature double precision,
    time timestamp with time zone
);

CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;
