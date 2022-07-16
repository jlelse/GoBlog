create table sections (
    blog text not null,
    name text not null,
    title text not null default '',
    description text not null default '',
    pathtemplate text not null default '',
    showfull boolean not null default false,
    primary key (blog, name)
);