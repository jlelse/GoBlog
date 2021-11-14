update posts set published = toutc(published), updated = toutc(updated);
-- insert into posts_fts(posts_fts) values ('rebuild');