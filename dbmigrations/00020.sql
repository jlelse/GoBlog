create table deleted (path text primary key);
create index index_post_parameters_par_val_pat on post_parameters (parameter, value, path);