create table if not exists media_optimized (
    original_hash text not null,
    variant_type text not null,
    optimized_hash text not null,
    width integer not null,
    height integer not null,
    primary key (original_hash, variant_type)
);