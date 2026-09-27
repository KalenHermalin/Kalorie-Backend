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

- [ ] **`AnalyzeFood` fails for every request that leaves `description` blank — confirmed live in
  production logs.**
  [internal/llm/gemini.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/llm/gemini.go#L35)
  (`AnalyzePicture`)
  **Symptom:** production logs show `Error 400 ... required oneof field 'data' must have one
  initialized field` from the Gemini API on `AnalyzeFood` calls. Since `description` is optional,
  this hits whenever a client sends an empty one — likely most requests.
  **Hint:** the `description` string always gets turned into its own `genai.Part{Text:
  description}` and appended unconditionally, even when it's empty. I reproduced this directly:
  marshaling a `genai.Part{Text: ""}` to JSON produces `{}` — a part with none of its oneof fields
  set, which the Gemini API rejects outright. Compare against how `systemPrompt` — a few lines
  below — builds its own `Part` slice just above this one; it doesn't have this problem. What does
  it do differently before including its `Part`?

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

- [x] ~~`GetUserFoodLogEntriesByLogId` never returns any rows — and has more than one thing wrong with it.~~
  All fixed: the outer slice was renamed to `entries` so it's no longer shadowed by the loop's
  `entry` variable, each scanned row is appended, the `deleted_at` scan now correctly passes
  `&entry.DeletedAt`, the scan error is checked, `rows.Err()` is checked after the loop, and
  `rows.Close()` is deferred. Matches `GetUserWeightLogs`'s pattern exactly now.
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L620). *(uncommitted)*

- [x] ~~Gemini calls ignore the request's timeout~~ — fixed in
  [internal/llm/gemini.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/llm/gemini.go). *(uncommitted)*

- [ ] **"No food/label found" is indistinguishable from a real server crash.**
  [internal/llm/gemini.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/llm/gemini.go#L82)
  (`AnalyzePicture`, and the equivalent spot in `AnalyzeLabel`)
  **Symptom:** point the camera at something that isn't food (or isn't a nutrition label) and the
  client gets back `ERR_INTERNAL_SERVER` / "An internal server error occured" — the exact same
  response a genuine crash would produce.
  **Hint:** when Gemini reports `success: false`, this function returns a plain `errors.New(...)`
  instead of an `*apperrors.AppError`. Trace what the handler's `errors.As` check does with a
  plain error versus an `AppError` — that's why this specific, extremely common case loses its
  identity by the time it reaches the client.

- [ ] **Exercise/food log sync reports partial failures as a full success.**
  [internal/handlers/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/user.go#L217)
  (`EgressSyncExerciseLogs`, and the identical spot in `EgressSyncFoodLogs`)
  **Symptom:** compare this branch against `EgressSyncWeightLogs`'s equivalent partial-failure
  case. Weight logs correctly send `207`. What status code does a client actually receive here
  when some (not all) exercise or food logs fail to sync?
  **Hint:** look for the `writer.WriteHeader(...)` call in this branch. Is there one?

- [ ] **Exercise/food log failures say "weight log."**
  [internal/service/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/user.go#L213)
  (and the mirrored code at `:312`, plus the `ERR_UPSERTING_WEIGHT_LOG` literal at `:218`/`:317`)
  **Symptom:** trigger a partial or total sync failure on `/api/v1/exercise-logs/sync` or
  `/api/v1/food-logs/sync` and read the `code`/`message` you get back closely.
  **Hint:** this whole function looks like it started as a copy of the weight-log sync function.
  What got copied that should have been renamed for this domain?

- [ ] **`ERR_GENERATING_ACCESS_TOKEN` is used for refresh-token failures too.**
  [internal/service/auth.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/auth.go#L92)
  (four spots total: `SignIn` and `RefreshAccessToken` each have one for the access token and one
  for the refresh token, and both use the same code)
  **Symptom:** a client branching on `error.code` to decide what to tell the user can't
  distinguish "we couldn't generate your access token" from "we couldn't generate your refresh
  token" — both come back with the identical code.
  **Hint:** read the `message` string right next to each of these four `NewAppError` calls versus
  the `code` string a few characters before it. Do they agree with each other?

- [ ] **A raw internal error string reaches the client.**
  [internal/service/auth.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/auth.go#L34)
  (`LogOut`)
  **Symptom:** compare this one `NewAppError` call against every other error message in this file
  and in `internal/handlers` — they're all fixed, hand-written strings. This one isn't.
  **Hint:** what gets concatenated onto the end of `"error deleting refresh token: "`? Where does
  that value come from, and is it something you'd want an end user to potentially see verbatim
  (a raw driver/SQL error string)?

- [ ] **The same "userId missing from context" bug means two different things depending on which
  endpoint hits it.**
  [internal/handlers/auth.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/auth.go#L94)
  (`HandleDeleteUser`, and `HandleLogOut` just above it) vs. any of the 8 identical checks in
  [internal/handlers/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/user.go#L30)
  **Symptom:** this exact same condition (the auth middleware failed to put a userId in the
  request context — should never happen in practice) produces `ERR_UNAUTHORIZED` / 401 / "please
  sign in again" in one file, and `ERR_MISSING_USER_ID` / 500 / an internal-error framing in the
  other. A client can't build one consistent response to this bug because it looks like two
  different bugs depending on which endpoint it hit.
  **Hint:** pick one of these two conventions and make both files agree with it.

---

## 🟡 Medium priority (gaps, consistency, docs)

- [x] ~~`FindRefreshToken` accepted an unused `userId` parameter~~ — resolved by removing the
  parameter entirely (interface, implementation, mock, and both call sites all updated
  consistently). *(uncommitted)*

- [x] ~~`/auth/refresh`'s `user.created_at` is always the same suspicious-looking date.~~ Fixed:
  `FindRefreshToken`'s query now selects `u.created_at` and scans it into `user.CreatedAt`.
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L30).
  First pass added the column to the `SELECT` but not to the matching `Scan(...)` call, which
  broke the endpoint entirely (`sql: expected 3 destination arguments in Scan, not 2`) — caught
  by actually running it, then fixed. Verified against a real Postgres: `FindRefreshToken` now
  returns the account's real `created_at`, matching what was set at signup exactly.
  [API.md](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/API.md) updated to drop the
  old zero-value-date note. *(uncommitted)*

- [x] ~~`UpsertUserExerciseSets` — synced weight changes don't stick.~~ Fixed: `weight =
  EXCLUDED.weight` added to the `DO UPDATE SET` list.
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L363).
  Verified against a real Postgres, not just by reading the SQL: upserted a set at weight 100,
  upserted the same id again at weight 150 with a later `updated_at`, fetched it back — came
  back as 150. *(uncommitted)*

- [x] ~~`GetUserExerciseLogs` — one query per log, not one query total.~~ Fixed: rewritten to
  fetch all parent logs, then all their sets in one `WHERE log_id = ANY($1)` query, grouped
  back onto their parent logs via a `map[string][]*models.ExerciseSet`.
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L263).
  Verified against a real Postgres: a log with 2 sets (one soft-deleted) grouped correctly, a
  log with 0 sets came back with an empty (not nil-panicking) slice, and a user with 0 exercise
  logs at all didn't error on the `ANY($1)` query with an empty id list. *(uncommitted)*
  Leftover: the stale `// TODO: Fix like done for the food logs` comment on line 262 now reads
  backwards — it's `GetUserFoodLogs` that needs to catch up to this one, not the other way
  around — worth updating/removing once `GetUserFoodLogs` gets the same treatment below.

- [x] ~~`GetUserFoodLogs` — one query per log, not one query total.~~ Fixed: same shape as the
  `GetUserExerciseLogs` fix above — all parent logs fetched, then all their entries in one
  `WHERE log_id = ANY($1)` query, grouped back via a `map[string][]*models.FoodLogEntry`.
  [internal/store/postgres_user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/store/postgres_user.go#L467).
  First pass didn't compile (`rows, err := ...` on the second query redeclared both vars already
  in scope from the first query, same file/function shape as the exercise-logs one avoided) —
  fixed. Verified against a real Postgres: a log with 2 entries (one soft-deleted) grouped
  correctly, a log with 0 entries came back with an empty (not nil) slice, and a user with 0 food
  logs at all didn't error on the `ANY($1)` query with an empty id list. `go build ./...` clean.
  *(uncommitted)*

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

- [ ] **`RemindMe` tells the user their request was invalid when the real problem is a server-side
  failure.**
  [internal/handlers/docs.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/docs.go#L59)
  **Symptom:** the duplicate-email case is handled specifically (409), but anything else that goes
  wrong storing the email — a DB error, a connection blip — falls back to `ERR_INVALID_REQUEST` /
  400, the same response a malformed JSON body gets.
  **Hint:** is a database failure actually the client's fault? What status code and code name would
  more accurately describe "we failed on our end," and how do other handlers in this codebase
  distinguish that case from a bad request?

- [ ] **A JSON field typed as Go's `error` interface — works today, but only by accident.**
  [internal/handlers/user.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/handlers/user.go#L196)
  (`SyncExerciseLogsResponse.Err`, and the identical field on `SyncFoodLogsResponse`)
  **Symptom:** not broken yet. I tested it directly: `json.Marshal` on this field only produces
  something useful (`{"code":..., "message":...}`) because the one error type ever assigned to it
  today (`apperrors.ErrSoftWeightLog`) happens to be an exported struct with its own JSON tags.
  **Hint:** what would `json.Marshal` produce for this field if some future code assigned it a
  plain `errors.New(...)` instead — the same kind of error several other functions in this
  codebase already return? (I checked: it's `{}` — silently empty, no code, no message.) Is a Go
  `error` interface the right type for a field that's going to be JSON-encoded and read by a
  client?

---

## 🟢 Low priority (cleanup, dead code, nice-to-haves)

- [x] ~~`internal/env/env.go`'s `GetString` unused~~ — package deleted entirely.
- [x] ~~`Nutrients`/`Portion`/`searchResponse` dead structs~~ — removed from `cmd/main.go`.
- [x] ~~`models.Provider` struct unused~~ — removed from `internal/models/user.go`.
- [ ] **Typos baked into shipped, user-facing error text and error codes.** Worth a pass before
  any of these are relied on by a client you can't update independently of this server —
  fixing the `message` text later is free, but changing a `code` string a client already
  branches on is a breaking change.
  - "presists"/"presist" instead of "persists" —
    [internal/service/auth.go:92](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/auth.go#L92),
    `:98`, `:104`, `:146`, `:153`
  - "yout" instead of "your" —
    [internal/apperrors/errors.go:106](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/apperrors/errors.go#L106)
    (`ErrSoftWeightLog`'s message)
  - Typos in the error *codes* themselves, not just messages —
    `ERR_DELETEING_USER` in
    [internal/service/auth.go:51](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/service/auth.go#L51)
    and `ERR_SOFT_UPSETING_WEIGHT_LOG` in
    [internal/apperrors/errors.go:105](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/apperrors/errors.go#L105)
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
- [x] ~~`heroku.yml` has no `release:` phase — migrations run inline on every dyno boot.~~ Fixed:
  migrations moved into a dedicated `cmd/migrate` binary
  ([internal/database/db.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/internal/database/db.go)
  now only connects/pings; `goose.SetDialect`/`goose.Up` live solely in
  [cmd/migrate/main.go](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/cmd/migrate/main.go)),
  [Dockerfile](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/Dockerfile) builds both
  `main` and `migrate` binaries into the same image, and
  [heroku.yml](https://github.com/KalenHermalin/Kalorie-Backend/blob/main/heroku.yml) runs
  `release: image: web, command: [./migrate]`. Verified by actually running `docker build` +
  `docker run` on the built image: both binaries present and executable, `./migrate` runs and
  correctly errors on a missing `DATABASE_URL` rather than failing to execute. Web dynos no
  longer touch `goose` at all, so this is also safe now if the dyno count ever goes above 1.
  *(uncommitted)*
- [ ] `AppleProvider.platform` is always `""` today. Not a problem — just noting it in case
  Apple's flow ever needs to diverge per-platform the way Google's does.
- [ ] Get real Apple Developer credentials configured (`APPLE_CLIENT_ID`, `APPLE_TEAM_ID`,
  `APPLE_KEY_ID`, `APPLE_PRIVATE_KEY`) whenever you're ready to actually enable Sign in with
  Apple — currently skipped at boot with a warning since these aren't set.
