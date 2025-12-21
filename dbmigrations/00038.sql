-- User authentication tables

-- Table for storing hashed password (single user system)
create table if not exists user_auth (
    id integer primary key check (id = 1), -- Only one user allowed
    password_hash text not null default ''
);

-- Initialize with empty password
insert or ignore into user_auth (id, password_hash) values (1, '');

-- Table for TOTP secrets
create table if not exists user_totp (
    id integer primary key check (id = 1), -- Only one user allowed
    secret text not null default ''
);

-- Initialize with empty TOTP secret
insert or ignore into user_totp (id, secret) values (1, '');

-- Table for multiple passkeys (WebAuthn credentials)
create table if not exists passkeys (
    id text primary key,
    name text not null default 'Passkey',
    credential text not null,
    created integer not null default (strftime('%s', 'now'))
);

-- Table for app passwords
create table if not exists app_passwords (
    id text primary key,
    name text not null,
    token_hash text not null,
    created integer not null default (strftime('%s', 'now'))
);
