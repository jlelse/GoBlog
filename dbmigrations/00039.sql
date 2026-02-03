-- Table for storing alternate domains for ActivityPub domain migration support
create table if not exists activitypub_alternate_domains (
    blog text not null,
    domain text not null,
    added integer not null default (strftime('%s', 'now')),
    primary key (blog, domain)
);
