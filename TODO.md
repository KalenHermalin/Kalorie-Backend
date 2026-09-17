# Kalorie Backend — TODO

Ordered urgent → not urgent within each section. Each open item gives you the **symptom** (what
you'd actually observe) and a **hint** (where to look / what question to ask yourself) — no code
fixes spelled out, so you can work out the actual change yourself. Links point at `main` on
GitHub — line numbers will drift as you edit, so if a link lands slightly off, the function name
in the title is the reliable anchor.

**Note:** several items below are checked off because they're genuinely fixed in your working
tree (verified each diff) but **not committed yet** — `git status` still shows them as modified.
Commit those whenever you're ready.

**Dead code removed in a previous pass:** the unused `internal/env` package, the leftover
`Nutrients`/`Portion`/`searchResponse` structs at the bottom of `cmd/main.go`, and the unused
`Provider` struct in `internal/models/user.go`. `middlewares.GetUserID`/`GetIsPremium` were
deliberately left even though they're also currently unused — see the low-priority item below.

---

## 🔴 Urgent

- [x] **Confirm the mobile app can actually log in and refresh end-to-end on production.** ✅ Done.

- [ ] **`go vet` / `go test ./internal/service` doesn't build.**
  [internal/service/auth_test.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/auth_test.go)
  **Symptom:** running `go vet ./...` or `go test ./...` fails before any test even runs, with a
  compile error naming `MockUserRepo` and a specific missing method, referencing
  `models.UserRepository`.
  **Hint:** an interface and one of its mock implementations have drifted apart. Line up
  `MockUserRepo`'s method list against the full interface it's supposed to satisfy — one method
  the interface requires has no stub on the mock. The other stub methods on `MockUserRepo` show
  you the shape a new one needs (signature, trivial return values).

---

## 🟠 High priority (real correctness bugs)

- [x] ~~8× missing `return` after the "user id missing from context" check~~ — fixed in
  [internal/handlers/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/user.go). *(uncommitted)*

- [x] **`UserService.UpsertUserWeightLogs` doesn't seem to ever report failure.**
  [internal/service/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/user.go#L84) (the weight-log sync function)
  **Symptom:** if something goes wrong inside the transaction this function runs (a DB error, a
  failed upsert), the caller still gets back a success result — no error, as if every log synced
  fine.
  **Hint:** this function calls a method that runs a block of code inside a database transaction
  and reports back whether that block succeeded or failed. Is that report actually being looked
  at here? Compare this function line-by-line against its two siblings just below it in the same
  file (the exercise-log and food-log sync functions) — they call the exact same kind of method.
  What's different about how they use its result?

- [x] ~~`GetUserExerciseLogById` panics~~ — fixed correctly: the target struct is now allocated
  before the `Scan` call, matching every sibling `Get*ById` method.
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L341). *(uncommitted)*

- [x] ~~`GetUserExerciseSetsByLogId` never returns any rows~~ — fixed, and fixed well: matches
  [`GetUserWeightLogs`](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L142)'s
  pattern exactly now (`defer rows.Close()`, per-row scan error checked, `rows.Err()` checked
  after the loop and returning its own value).
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L399). *(uncommitted)*

- [ ] **`GetUserFoodLogEntriesByLogId` never returns any rows — and has more than one thing wrong with it.**
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L608)
  **Symptom:** same empty-list symptom as above. If you go looking for "where does it append to
  the result slice" the way you would for the previous bug, you'll find something that looks
  like it should work but doesn't — that's a second, sneakier issue layered on top of the first.
  There's also a subtle scan issue on the very last column, similar in flavor to a bug elsewhere
  in this same file where a nullable timestamp column is scanned incorrectly.
  **Hint:** two separate questions to answer here: (1) the variable declared *inside* the loop —
  does its name collide with something declared *outside* the loop? If so, which one does Go
  actually treat as "the same variable" at each point in the function, and which one gets
  returned at the very end? (2) for the `deleted_at` column specifically, compare how it's passed
  to `Scan` here versus how every other nullable timestamp column is passed to `Scan` anywhere
  else in this file — one extra character is missing.

- [x] ~~Gemini calls ignore the request's timeout~~ — fixed in
  [internal/llm/gemini.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/llm/gemini.go). *(uncommitted)*

---

## 🟡 Medium priority (gaps, consistency, docs)

- [x] ~~`FindRefreshToken` accepted an unused `userId` parameter~~ — resolved by removing the
  parameter entirely (interface, implementation, mock, and both call sites all updated
  consistently). *(uncommitted)*

- [ ] **`/auth/refresh`'s `user.created_at` is always the same suspicious-looking date.**
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L30) (`FindRefreshToken`)
  **Symptom:** hit `/auth/login` and `user.created_at` is a real, sensible date. Hit
  `/auth/refresh` for the same account and `user.created_at` comes back as
  `0001-01-01T00:00:00Z`. Already documented in
  [API.md](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/API.md) as current
  behavior — this is a "decide if you care" item, not a clear-cut bug.
  **Hint:** compare exactly which columns get selected out of the `users` table in the query this
  function runs vs. the query the login path's equivalent function runs. Decide: is fetching one
  more column worth it for a field this endpoint doesn't strictly need?

- [ ] **`UpsertUserExerciseSets` — synced weight changes don't stick.**
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L364) (the `INSERT ... ON CONFLICT` statement)
  **Symptom:** sync an exercise set, then sync the *same* set id again later with a different
  weight (and a newer `updated_at`, so it's not being rejected as stale). Reps and set number
  update fine. Weight silently stays at whatever it was the first time.
  **Hint:** look at the full list of columns the "on conflict, update these" clause actually
  updates, versus the full list of columns being inserted. One column is present in the insert
  but absent from the update list — is that intentional?

- [ ] **`GetUserExerciseLogs` / `GetUserFoodLogs` — one query per log, not one query total.**
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L258)
  has a `// TODO: Fix like done for the food logs` comment sitting on it — but
  [`GetUserFoodLogs`](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L452)
  has the identical shape, so there's no "done" version to copy from.
  **Symptom:** not a correctness bug — a scaling one. A user with 50 exercise logs triggers 51
  separate database round-trips on a single sync request.
  **Hint:** both functions fetch the parent rows, then loop over them fetching each one's
  children individually. Could the children for *all* parents be fetched in one single query
  instead (there's a SQL construct for "give me all rows whose foreign key is in this list of
  ids"), then matched back up to their parents afterward in Go?

- [ ] **Inconsistent handling of "no row found" across similar methods.**
  Compare
  [`GetUserSettings`](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L176) /
  [`GetUserWeightLogById`](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L113)
  against `FindRefreshToken`, `GetUserFoodLogById`, and `GetUserExerciseLogById`.
  **Symptom:** not currently breaking anything visible — a maintainability/consistency question.
  **Hint:** when a query finds zero matching rows, some of these methods translate that into a
  specific sentinel error the rest of the codebase can recognize; others just hand back
  whatever the database driver itself returned. Worth picking one convention.

- [ ] **No tests exist for `GitHubProvider` / `GoogleProvider` / `AppleProvider`.**
  [internal/auth/providers/](https://github.com/KalenHermalin/Kalorie-Backend/tree/main/internal/auth/providers)
  **Symptom:** the recent fix to GitHub/Google's silent-failure handling has no automated test
  guarding it — a future edit could reintroduce that bug and nothing would catch it. Apple's
  client-secret-signing and id_token-verification logic are also completely unexercised by any
  test today.
  **Hint:** GitHub/Google currently call fixed, hardcoded URLs (like
  `https://api.github.com/user`) directly inside their methods. What would have to change about
  where those URLs come from for a test to be able to point them at a fake local server instead?
  Separately: which parts of `AppleProvider` need *zero* network access to test right now,
  as-is?

- [x] ~~`API.md` said the app was deployed on DigitalOcean~~ — fixed, now says Heroku with the
  correct base URL. *(uncommitted)*

- [ ] **`API.md`'s exercise-logs and food-logs sync sections describe a different convention than weight-logs now.**
  Not inaccurate — they correctly describe what their own endpoints still actually do. The
  weight-log sync endpoint was redesigned (independent per-log transactions, `failed_logs` as a
  list of ids instead of full objects, a `207` for partial success, a 200-item batch cap, no more
  `error` field) and its docs were updated to match; exercise-logs and food-logs haven't been
  touched, so their sections of
  [API.md](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/API.md) still correctly
  describe the older shape. Flagged so it's tracked as a known, deliberate, temporary
  inconsistency rather than forgotten. Decide: bring exercise-logs/food-logs in line with the new
  weight-logs pattern, or decide they're fine staying different and drop this item.

- [ ] **Duplicated boilerplate across the 4 sync handler pairs.**
  [internal/handlers/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/user.go) —
  settings, weight logs, exercise logs, and food logs each have an egress and ingress handler.
  **Symptom:** not a bug — a maintenance cost. The same "parse this query parameter or fail with
  400" logic and the same "unwrap this error into the right HTTP response" logic appears, nearly
  identically, 8 separate times.
  **Hint:** read all 4 ingress handlers side by side. What's byte-for-byte identical between all
  of them? Same question for the error-unwrapping block that appears near the end of every one
  of the 8 handlers.

---

## 🟢 Low priority (cleanup, dead code, nice-to-haves)

- [x] ~~`internal/env/env.go`'s `GetString` unused~~ — package deleted entirely.
- [x] ~~`Nutrients`/`Portion`/`searchResponse` dead structs~~ — removed from `cmd/main.go`.
- [x] ~~`models.Provider` struct unused~~ — removed from `internal/models/user.go`.
- [ ] `middlewares.GetUserID` / `GetIsPremium` are still unused by any handler (handlers read
  the context value directly instead of calling these). Deliberately kept rather than deleted —
  they're a correct, intentional accessor pattern, not abandoned code. Decide: start using them
  where handlers currently do the equivalent inline, or remove them.
- [ ] [internal/apperrors/errors.go:34](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/apperrors/errors.go#L34) —
  a TODO says app errors should only be constructed in the service layer. Worth checking whether
  any handler currently breaks that rule, and whether it actually matters.
- [ ] [internal/models/auth.go:27](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/models/auth.go#L27) —
  a TODO about a code-exchange variant that doesn't need a PKCE verifier. Only worth revisiting
  if a future provider never uses PKCE.
- [ ] [internal/service/llm.go:13](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/llm.go#L13) —
  a TODO about unifying the food/label analysis path. Look at
  [internal/handlers/llm.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/llm.go) —
  how much of the two handlers is actually different from each other?
- [ ] [internal/llm/gemini.go:36](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/llm/gemini.go#L36) —
  a TODO about shared setup between `AnalyzePicture` and `AnalyzeLabel`. Same question: how much
  of the two functions is identical setup code before they diverge?
- [ ] Decide what to do with the abandoned `port-python-fastapi` branch on GitHub. Keep it,
  tag it for the record, or delete it.
- [ ] `heroku.yml` has no `release:` phase — migrations run inline on every dyno boot, which is
  only a real problem once you're running more than one web dyno at a time (simultaneous boots
  could race on a brand-new migration file). Not a current issue; worth remembering if the dyno
  count ever changes.
- [ ] `AppleProvider.platform` is always `""` today. Not a problem — just noting it in case
  Apple's flow ever needs to diverge per-platform the way Google's does.
- [ ] Get real Apple Developer credentials configured (`APPLE_CLIENT_ID`, `APPLE_TEAM_ID`,
  `APPLE_KEY_ID`, `APPLE_PRIVATE_KEY`) whenever you're ready to actually enable Sign in with
  Apple — currently skipped at boot with a warning since these aren't set.
