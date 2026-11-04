-- GET /links lists newest first.
CREATE INDEX links_created_at_idx ON links (created_at DESC, id DESC);
