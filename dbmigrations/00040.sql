create table if not exists webmention_blocklist (
    host text not null primary key,
    incoming integer not null default 0,
    outgoing integer not null default 0
);
