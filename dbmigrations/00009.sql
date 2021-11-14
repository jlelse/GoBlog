alter table posts add status text;
update posts set status = 'published';
drop view view_posts_with_title;
drop table posts_fts;
create view view_posts_with_title as select id, path, title, content, published, updated, blog, section, status from (select p.rowid as id, p.path as path, pp.value as title, content, published, updated, blog, section, status from posts p left outer join post_parameters pp on p.path = pp.path where pp.parameter = 'title');
create virtual table posts_fts using fts5(path unindexed, title, content, published unindexed, updated unindexed, blog unindexed, section unindexed, status unindexed, content=view_posts_with_title, content_rowid=id);
-- insert into posts_fts(posts_fts) values ('rebuild');