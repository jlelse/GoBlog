drop table posts_fts;
drop view view_posts_with_title;
create view posts_fts_view as select p.rowid as id, p.path as path, coalesce(pp.value, '') as title, p.content as content from posts p left outer join (select * from post_parameters pp where pp.parameter = 'title') pp on p.path = pp.path;
create virtual table posts_fts using fts5(path unindexed, title, content, content=posts_fts_view, content_rowid=id);
insert into posts_fts(posts_fts) values ('rebuild');