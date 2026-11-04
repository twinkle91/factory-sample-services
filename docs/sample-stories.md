# Sample stories for the Software Factory

Copy each block into a Jira issue. Each story has a summary, a description and
acceptance criteria, and says which Factory path it exercises.

---

## SL-1 — Links can expire

**Type:** Story · **Exercises:** a new migration, PostgreSQL integration tests

**Description**

As someone sharing a link for a campaign, I want it to stop working after a
date, so old links don't keep sending people to outdated pages.

`POST /links` accepts an optional `expires_at` (RFC 3339 timestamp). Once that
time has passed, the link is treated as gone.

**Acceptance criteria**

- A new migration `003_add_links_expires_at.sql` adds a nullable
  `expires_at timestamptz` column to `links`. Existing links have no expiry.
- `POST /links` accepts `{"url": "...", "expires_at": "2026-12-31T23:59:59Z"}`.
  An `expires_at` in the past, or one that is not RFC 3339, returns `400`.
- `GET /links/{code}` on an expired link returns `410 Gone` with
  `{"error": "link expired"}` and does **not** add a visit.
- `GET /links/{code}/stats` still works for expired links.
- Responses include `expires_at` (omitted or `null` when there is none).
- `GET /links` keeps listing expired links.
- Integration tests cover an expired link, a future expiry and no expiry
  against the real database.

---

## SL-2 — Rate limit link creation per client IP

**Type:** Story · **Exercises:** Redis test service

**Description**

Someone scripted thousands of `POST /links` calls yesterday. Limit how many
links one client can create.

**Acceptance criteria**

- Each client IP may create at most **10 links per minute** (a fixed window is
  fine). The limit is shared by every server instance, so it lives in Redis,
  with keys under the existing `shortlinks:` prefix that expire on their own.
- The client IP is the host part of `RemoteAddr`. Do not trust
  `X-Forwarded-For` yet.
- Over the limit, `POST /links` returns `429 Too Many Requests` with a
  `Retry-After` header (seconds until the window resets) and
  `{"error": "rate limit exceeded"}`.
- Requests rejected by validation (`400`) still count toward the limit.
- Other endpoints are not limited.
- If Redis is down, `POST /links` returns `503` rather than skipping the limit.
- Integration tests against the real Redis show the 11th request in a window is
  rejected and that two different IPs have separate limits.

---

## SL-3 — Delete a link

**Type:** Story · **Exercises:** a small endpoint across PostgreSQL and Redis

**Description**

Let us remove a link that was created by mistake.

**Acceptance criteria**

- `DELETE /links/{code}` removes the link and returns `204 No Content`.
- Its visit counter in Redis is removed too.
- Deleting an unknown code returns `404` with `{"error": "link not found"}`.
- After deletion, `GET /links/{code}` and `GET /links/{code}/stats` return `404`
  and the link no longer appears in `GET /links`.
- Unit and integration tests cover the new endpoint.

---

## SL-4 — Reject links to localhost and private networks

**Type:** Story · **Exercises:** a validation change with unit tests

**Description**

Short links pointing at `localhost` or internal addresses could be used to
probe our network from tools that follow them. Reject them when the link is
created.

**Acceptance criteria**

- `POST /links` returns `400` with
  `{"error": "url must not point to a private or local address"}` when the host is:
  - `localhost` or any name ending in `.localhost`;
  - an IPv4 loopback (`127.0.0.0/8`), private (`10/8`, `172.16/12`,
    `192.168/16`) or link-local (`169.254/16`) address;
  - IPv6 `::1`, unique local (`fc00::/7`) or link-local (`fe80::/10`).
- Hostnames are **not** resolved through DNS; only literal names and IPs are
  checked.
- Public hosts such as `https://example.com` and `https://8.8.8.8` are still
  accepted.
- Unit tests in `internal/links` cover each rejected range and a few accepted
  hosts.

---

## SL-5 — Clean up old links

**Type:** Story · **Exercises:** the planner asking a clarifying question

> This story is deliberately vague. A good plan asks what "old" and "clean up"
> mean before writing code.

**Description**

The links table keeps growing. We should clean up old links so the service
stays fast.

**Acceptance criteria**

- Old links are cleaned up regularly.
- Nothing important is lost.

*(Left open on purpose: is "old" about age, or about no visits? Is the link
deleted or archived? What happens to its Redis counter? Is it a background job,
a startup task or an admin endpoint? How often does it run?)*

---

## Bug report

### SL-6 — `GET /links` ignores the 100-link maximum

**Type:** Bug · **Priority:** Medium · **Exercises:** the regression-test-first flow

**Description**

The API documentation (README, `GET /links`) and the `MaxLimit` constant in
`internal/links/validate.go` say `GET /links` returns at most 100 links.
In practice `?limit=` is passed straight to the database, so any client can ask
for every row in one request. On a large table that is a slow query and a very
large response.

**Steps to reproduce**

1. Start the server with PostgreSQL and Redis.
2. Create 120 links with `POST /links`.
3. Call `GET /links?limit=500`.

**Expected:** 100 links (the newest 100).

**Actual:** all 120 links.

**Notes**

- `?limit=` values from 1 to 100 behave correctly, and invalid values
  (`0`, `-3`, `ten`) already return `400`. That is why the existing tests pass.

**Acceptance criteria**

- First, a failing test shows `?limit=` above 100 returning more than 100 links.
- `?limit=` above 100 returns 100 links. It is capped silently, not rejected.
- Existing behaviour for missing, valid and invalid limits is unchanged.
