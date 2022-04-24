-- Add foreign key constraint to post_parameters table
create table post_parameters_new (
    id integer primary key autoincrement,
    path text not null,
    parameter text not null,
    value text not null default '',
    foreign key (path) references posts(path) on update cascade on delete cascade
);
drop view posts_fts_view;
create view posts_fts_view as select p.rowid as id, p.path as path, coalesce(pp.value, '') as title, p.content as content from posts p left outer join (select * from post_parameters_new pp where pp.parameter = 'title') pp on p.path = pp.path;
insert into post_parameters_new select * from post_parameters;
drop table post_parameters;
alter table post_parameters_new rename to post_parameters;
create index index_post_parameters on post_parameters (path, parameter, value);
create index index_post_parameters_par_val_pat on post_parameters (parameter, value, path);
insert into posts_fts(posts_fts) values ('rebuild');