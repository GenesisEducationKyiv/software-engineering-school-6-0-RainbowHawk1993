create table if not exists subscriptions (
    id bigserial primary key,
    email text not null,
    repo_owner text not null,
    repo_name text not null,
    confirmed boolean not null default false,
    confirm_token text not null unique,
    unsubscribe_token text not null unique,
    last_seen_tag text not null default '',
    confirmed_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (email, repo_owner, repo_name)
);

create index if not exists subscriptions_confirmed_repo_idx
    on subscriptions (confirmed, repo_owner, repo_name);

create index if not exists subscriptions_email_idx
    on subscriptions (email);
