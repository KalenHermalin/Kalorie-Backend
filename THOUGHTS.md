# Thoughts

Open design questions to think through later. Not tasks — those live in `TODO.md`.

---

## Exercise sync merges at the set level, not the log level

The exercise log upsert and its sets upsert each have their own `updated_at` check. A set is
written whenever *it* is newer than the server's copy of that set, even if the log update in the
same request was skipped as stale.

The client rule "editing a set also bumps the log's `updated_at`" holds per device, but doesn't
cover two devices:

1. Device B edits the log at 3:00 and syncs. Server log is now 3:00.
2. Device A, offline, edited a set at 2:00 and bumped *its* log to 2:00. It syncs.
3. A's log (2:00) is older than the server's (3:00), so the log update is skipped. A's set (2:00)
   is newer than the server's copy of *that* set (B never touched it), so the set is written.

Result: A's set edit and B's log edit both survive. A merge, not "newest log wins".

Questions:
- Is a per-set merge the intended behaviour? (Probably yes — no edit is lost.)
- What if B's 3:00 change was *deleting* the log? A's set edit lands under a deleted log.
- Should food logs and their entries follow the same rule?

---

## Conflict detection relies on client rules the server doesn't enforce

`UpsertUserExerciseLog` decides "conflict or not" from `RowsAffected`, which only counts sets
written (the log upsert is inside the `WITH`). That's correct only if the client guarantees:
- a log is never sent with zero sets
- a log is only ever updated together with at least one set (same `updated_at`)

Questions:
- How does soft-deleting a log work under these rules? If it bumps the log but no set, it looks
  like a conflict.
- The two-device case above reports success while the log was stale. Fine because the next
  ingress pull corrects it?
- Should the server reject requests that break the rules, or just document them in `API.md`?

---

## Sync the parent log and its children on one timestamp

Idea: treat a log and all of its sets (or entries) as a single unit with one `updated_at`, so the
server makes one newer/older decision per log instead of one per row.

To think about:
- Which timestamp is the source of truth, and what happens to the per-set `updated_at` columns?
- If the log is newer, does the server replace *all* of its sets? What about sets the client
  removed (not in the payload)?
- If the log is older, every set in the request is skipped. That gives up the two-device merge
  above ("newest log wins" instead). Is that acceptable?
- Does the client then have to send the full set list every time, not just changed sets?
- Same question for `food_logs` / `food_log_items`.

**Recommendation (2026-10-27): go with one timestamp per log, whole log wins.**
- The log's `updated_at` is the only version. Newer → save the log and replace its set list;
  older → skip everything and return the server's full log in `new_logs`.
- Only loses data when one user edits the same log on two offline devices before either syncs —
  rare for a single-user app, and "most recent edit wins" is what users expect.
- Removes the `RowsAffected` / client-rule problems: the conflict signal comes from the log row.
- Logs are small, so sending the full set list each time is cheap.
- Main thing to design: what "a set missing from the list" means (soft delete vs hard delete).
- Revisit if logs ever become shared between users or get large.
