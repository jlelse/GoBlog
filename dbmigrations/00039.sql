alter table posts add column wordcount integer not null default 0;
alter table posts add column charcount integer not null default 0;
update posts set wordcount = wordcount(mdtext(coalesce(content, ''))), charcount = charcount(mdtext(coalesce(content, '')));
delete from persistent_cache where key like 'blogstats_%';
