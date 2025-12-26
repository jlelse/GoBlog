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
    hash text not null,
    created integer not null default (strftime('%s', 'now'))
);
