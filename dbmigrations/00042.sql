create table fediverseapps (
  id text primary key,
  name text not null,
  secret text not null,
  redirect_uris text not null,
  scopes text not null,
  website text,
  created integer not null
);
