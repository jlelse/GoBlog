drop table sessions;
create table sessions (id text primary key, data blob, created text default '', modified text default '', expires text default '');
create index sessions_exp on sessions (expires);