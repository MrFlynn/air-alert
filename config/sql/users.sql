drop table if exists users;

create table users (
    id serial not null primary key,
    push_url text not null,
    private_key text not null,
    public_key text not null,
    longitude double precision not null,
    latitude double precision not null,
    threshold double precision not null,
    last_crossover timestamp with time zone
);