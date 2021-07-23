create table indieauthauthnew (time text not null, code text not null, client text not null, redirect text not null, scope text not null);
insert into indieauthauthnew (time, code, client, redirect, scope) select time, code, client, redirect, scope from indieauthauth;
drop table indieauthauth;
alter table indieauthauthnew rename to indieauthauth;
create table indieauthtokennew (time text not null, token text not null, client text not null, scope text not null);
insert into indieauthtokennew (time, token, client, scope) select time, token, client, scope from indieauthtoken;
drop table indieauthtoken;
alter table indieauthtokennew rename to indieauthtoken;