drop view view_posts_with_title;
create view view_posts_with_title as select p.rowid as id, p.path as path, coalesce(pp.value, '') as title, content, published, updated, blog, section, status from posts p left outer join (select * from post_parameters pp where pp.parameter = 'title') pp on p.path = pp.path;
-- insert into posts_fts(posts_fts) values ('rebuild');