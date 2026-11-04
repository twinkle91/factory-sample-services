CREATE TABLE links (
    id         bigserial   PRIMARY KEY,
    code       text        NOT NULL UNIQUE,
    url        text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
