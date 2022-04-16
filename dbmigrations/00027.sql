create table reactions (
    path text not null,
    reaction text not null,
    count integer default 0,
    primary key (path, reaction),
    foreign key (path) references posts(path) on update cascade on delete cascade
);