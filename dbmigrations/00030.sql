create table settings (
    name text not null,
    value text not null default '',
    primary key (name)
);