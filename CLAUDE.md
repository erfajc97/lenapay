# Lenapay — CLAUDE.md

Fintech wallet (CVU accounts ARS/USD) for Lebane's clients and, later, individuals.
Monorepo: `apps/api` (Go), `apps/bind-mock` (Go, fake PSP), `apps/mobile` (Expo),
`contracts/openapi.yaml` (single API contract), `packages/api-types` (generated TS types).
Read first: `docs/prd.md`, `docs/architecture.md`, `docs/backlog.md`, `docs/brand.md`.

**Rigor: lite.** Enforced: architecture/structure, secrets, no dead code, and the money rules
(MO + ledger invariants) with real tests. Everything else below is guidance. Only `/kit:audit
quick` runs after tasks. Builders never commit; the main agent commits locally with
`[ADD]/[FIX]/...` subjects. Never push without the owner's OK.

Project rules:
- P1. Contract first: any API change starts in `contracts/openapi.yaml`, then `make gen`.
- P2. Every external dependency (PSP/BIND, KYC, ERP, IdP, email/push) is a port with a fake;
  local runs never call real services.
- P3. Code, tests, comments and commits in English; UI copy in es-AR through i18n files.
- P4. The ledger is the only writer of balances; money is int64 minor units + currency.
- P5. Feature packages own their files; cross-feature calls go through exported service
  interfaces wired in `cmd/api/main.go`.

<!-- kit:block general v1 -->
## Rules — General (`G`)

Every rule is cited by ID in reviews and commits. Auditors check them with the command or
pattern given; "Grep" means the Grep tool (ripgrep syntax), never `grep` through a shell.

G1. **Plan before code.** Every task exists in `docs/backlog.md` with its requirement ID
    (`R-<AREA>-<n>`), dependencies and acceptance criteria before code is
    written; a scope change is written there first. Why: work that is not in the plan is not
    audited against a requirement and drifts. Check: the task ID quoted by the builder and the
    commit appears in `docs/backlog.md` (Grep the ID).

G2. **TDD.** Test first (red), minimum code (green), refactor. No logic file without its
    spec, and every acceptance criterion becomes an automated test. A bug fix brings a
    regression test that FAILS on the old code; a security or money fix also brings the
    attacker test (the exact request that exploits it) and the third-party-down test. A
    refactor without behaviour change runs the new spec against the OLD code first. The
    coverage gate (70 % lines and branches on logic folders) lives in each runner
    config and is never lowered to make a change pass. Why: "there are tests" is not
    evidence; a test that was never red proves nothing. Check: for every file added or
    changed under a logic folder (`services/`, `hooks/`, `helpers/`, `mutations/`,
    `*.service.*`, guards, validators, jobs) a spec exists — `git diff HEAD --name-only`,
    `git ls-files --others --exclude-standard`, then Glob the sibling `__tests__/` or
    `*.spec.*`/`*.test.*`; the runner exits 0 with the gate enabled.

G3. **Mutation check, not "there are tests".** Every new or changed rule is broken on
    purpose — invert a comparison, `>` → `>=`, delete a guard, return a constant, drop the
    tenant/owner filter, skip the transaction — and some test must fail. Mutants run in
    memory (the Vitest transform recipe in `.claude/kit/mutant/`, the default). Only the
    recipe's copy-and-restore fallback (Jest, or a runner without transforms) may write the
    mutant into the file, and only while no dev server, watcher or other agent reads it:
    backup first, restore in a `finally`; the source's sha256 is identical before and after. A
    mutation run only counts on a green baseline. A surviving mutant means the TEST is
    fixed (or the commit says why that mutant cannot be killed). Report `N/N killed`. The
    fake-test patterns `FT1…FT22` below are cited by number. Why: in the source projects
    half of the first mutants survived green suites. Check: the builder report has the
    `rule → mutant → killing test` table; `auditor-tests` re-runs a sample with
    `.claude/kit/mutant/mutant.mjs`.

G4. **No dead code.** No unused exports, commented-out code, unreachable branches,
    duplicated logic, or `TODO`/`FIXME` in added lines; a refactor deletes the old path in
    the same commit. Why: dead paths get "fixed" later and diverge from the live one.
    Check: `/kit:audit quick` flags commented-out code, TODOs and exports without importers;
    by hand, Grep the added lines (`git diff HEAD -U0`) for
    `^\+\s*//\s*(const|let|var|if|for|return|await|export|import|function|class)\b`.

G5. **Readable.** Names say what the data represents; a function that does not fit a
    screen (~50 lines) or nests more than 3 levels is split; more than 2 parameters → one
    object in, one object out. Comments explain WHY, never WHAT; a test says why the rule
    matters (the bug or the money it protects). Why: the next agent reads this cold.
    Check: read the touched functions; report file:line and the simpler shape.

G6. **Audit at the project's rigor — automatically.** This project runs at rigor
    **`lite`**. After EVERY finished task and before EVERY commit, the main agent runs
    the audit of that level without being asked:

    | Rigor | After each task and before each commit | Milestones |
    |---|---|---|
    | `full` | `/kit:audit` — the whole `audit.chain` for the touched scopes | `auditor-judgment` + `auditor-senior` |
    | `standard` | `/kit:audit` — `rules` + `tests`; the rest of the chain on demand (`/kit:audit full`) | on demand |
    | `lite` | `/kit:audit quick` — architecture, secrets and dead code (`arch-check`, no agents) | none |

    In `lite`, the ENFORCED rules are the architecture rules of the scope blocks (layout,
    layers, imports — whatever `arch-check` checks), G9 (secrets) and G4 (dead code); every
    other rule is guidance, tests are written where cheap and G3 is skipped. In every
    level nothing is "done" with an open BLOCKER or HIGH, and after fixes only the failed
    audits re-run. With `"seal": true` the kit hook blocks `git commit` until the staged
    files match the seal of a PASS. Why: the author cannot see what the author missed.
    Check: the last audit verdict is PASS for the exact paths committed
    (`.claude/kit-seal.json` when the seal is on).

G7. **Git writes by the main agent only.** Builders and auditors never run `git add`,
    `commit`, `stash`, `checkout`, `restore`, `reset`, `rebase` or `push`. Commit message:
    [TAG] short subject; the body says WHY, then a `Verified:` line (commands and counts) and
    `Mutation: N/N`, plus a `Deploy order:` line when a contract changes. The message goes
    in a file (`git commit -F <file>`); staging is its own command; `--no-verify` and
    `HUSKY=0` are forbidden. Why: several agents share one tree; one stray `checkout`
    destroys uncommitted work. Check: `git log -1 --format=%B` matches the format; agent
    reports contain no git writes.

G8. **Tracking.** When a task closes, update docs/backlog.md and docs/progress.md: the task status, the
    checklist row with its evidence (file:line, test name or command output) and
    docs/progress.md. Exactly one progress log is the source of truth. Why: progress kept
    from memory is wrong by the second day. Check: the closed task's ID is ticked in
    docs/progress.md and its checklist row has evidence, in the same commit.

G9. **Secrets.** Never in code, tests, fixtures, logs, docs, the client bundle or git.
    `.env` is git-ignored; every variable is declared in the env schema validated at boot
    (production fails closed without it), in `.env.example` with a placeholder, and in the
    env doc. Only public-prefixed variables reach a client bundle. Why: a leaked key is
    rotated, not deleted. Check: `/kit:audit quick` (tracked env files, provider keys,
    secret literals); `git ls-files '*.env' '*.env.*'` lists only `*.example` files.

G10. **Verify end to end.** Before saying "done": lint, typecheck, tests and build of every
     touched scope (the commands in `kit.json` → `commands.<scope>`), reading each EXIT
     CODE, not the summary; then the golden path in a real browser or device at the
     smallest supported viewport and at desktop. Why: green unit tests have shipped broken
     pages. Check: the builder report lists each command with its exit code
     (`<cmd>; echo "exit=$?"`).

G11. **Ambiguous or contradictory → ask, don't invent.** Write the question in
     docs/progress.md with a proposed option marked `(proposal)`; if it changes scope, data
     model, cost or security, ask the owner before building. `docs/prd.md` wins
     over the checklist and the plan; report any discrepancy. Why: an invented requirement
     is a bug with good test coverage. Check: every `(proposal)` decision in the change is
     logged; no behaviour exists that no requirement or decision asks for.

G12. **One rule, one place.** A business rule (visibility, eligibility, a cap, a price
     calculation) lives in ONE function used by every door that applies it, including the
     one that charges; a rule both front and back need lives once in a shared package.
     "Twin doors": every DTO or form that receives the same field applies the same cap and
     normalization, pinned by a test that compares them; a cap shared by API and UI is one
     constant that the copy interpolates. Every business classification of an enum has a
     spec against the schema enum. Why: the second copy is the one that does not get the
     fix. Check: for each new/changed field, condition or number, Grep its name or literal
     across all scopes; two implementations is a finding.

G13. **Language.** Code, identifiers, comments, commits and docs in English. End-user copy
     (`es-AR` by default) only through the i18n layer (`t('ns.key')`), never
     hard-coded — also `aria-label`, `placeholder`, `title`, toasts, emails, notifications
     and error copy; every key exists in every locale. Why: hard-coded copy cannot be
     translated or reviewed in one place. Check: Grep touched `.tsx`/`.astro`/`.vue` files
     for `>\s*[A-Za-zÁÉÍÓÚáéíóúñÑ¿¡][^<{}]*<` and
     `(aria-label|placeholder|title|alt)=["'][^"'{]+["']`; diff the key sets of the locale
     files.

G14. **Multi-agent hygiene.** Each agent touches only the paths of its task. Shared files
     (error catalog, i18n locales, DB schema, module registries, env schema) are edited
     surgically — append, never rewrite or reorder. Before creating a "new" file, Glob and
     `git ls-files` for it; search with the Grep tool. A batch's state lives in a file that
     is enough to resume after a cut; once a batch closes its scope is frozen (new findings
     go to the backlog, except a HIGH in security or money). Why: parallel agents silently
     overwrite each other. Check: `git diff HEAD --stat` of shared files shows additions
     only; no file outside the task's paths changed.

G15. **Quality standard.** Rules learned the hard way live in `docs/quality-standard.md`:
     each entry tells the real bug, the correct way, how it is enforced (rule ID, census or
     test) and cites the commit that taught it. The same defect found twice becomes an
     entry; auditors read the file before auditing. Why: fixes were applied to old code and
     the next feature repeated the mistake. Check: every entry has a commit hash
     (`git cat-file -t <hash>` → `commit`) and an enforcement line.

G16. **Censuses only shrink.** A mechanical check that needs the whole repo is a script
     `scripts/census-<name>.mjs` (read-only, never opens `.env`, no network) that exits 1 on
     any NEW occurrence. Pre-existing occurrences sit in its `BASELINE` list, each with a
     reason; an entry that no longer occurs also fails, so the baseline only shrinks. Each
     census has a `node --test` spec with synthetic cases that must bite. A rule that is a
     layout, import or naming rule goes to `.claude/kit-structure/*.structure.json` instead,
     so `/kit:audit quick` checks it. Why: a rule that only lives in prose is broken by the
     next feature. Check: run every `scripts/census-*.mjs` and read the exit code.

G17. **No invisible control characters.** No raw bidi controls, zero-width spaces or
     direction marks in any text file (code, specs, docs, JSON, YAML); code that needs one
     builds it with `String.fromCodePoint`. Why: the text reads differently from what it
     executes ("trojan source"). Check: `/kit:audit quick`; by hand, Grep
     `[\x{202A}-\x{202E}\x{2066}-\x{2069}\x{200B}\x{200E}\x{200F}]` → zero hits.

G18. **No silent failure.** Every `catch` handles the error, reports it, or says in a
     comment why neither applies; no empty `catch {}`, no `except: pass`, no promise left
     floating (`void promise()` without `.catch`). Why: a swallowed error turns an outage
     into a support ticket. Check: Grep added lines for
     `catch\s*(\(\w*\))?\s*\{\s*\}`, `\.catch\(\(\)\s*=>\s*(\{\s*\}|undefined|null)\)`,
     `except(\s+\w+)?:\s*pass` and `\bvoid\s+\w+\(`.

G19. **pnpm, locked down (supply chain).** Every JavaScript package installs with pnpm
     ≥ 10.26 (the first version that reads `allowBuilds`, the only build allowlist pnpm 11
     and later still read). The root `package.json` pins it exactly in `packageManager`
     (`"pnpm@<x.y.z>"`) and every machine and CI job runs that version: pnpm switches to
     the pin by itself (`pmOnFail: download`, the default from pnpm 11), or Corepack does
     (`corepack enable`; bundled with Node.js only before 25, `npm i -g corepack` after).
     `pnpm-lock.yaml` is committed and CI installs with `pnpm install --frozen-lockfile`.
     Nobody runs `npm install` or `yarn` here: no `package-lock.json` or `yarn.lock` exists.
     Supply-chain settings live in `pnpm-workspace.yaml` (from pnpm 11, `.npmrc` holds only
     auth and registry settings and the rest of it is ignored). Dependency lifecycle scripts
     never run unless allowed (blocked by default since pnpm 10.0): each package that needs
     its build script is listed `true` in `allowBuilds` (`pnpm approve-builds`, since 10.1,
     writes it; `onlyBuiltDependencies` is its pre-10.26 spelling, removed in pnpm 11) with a
     comment saying why, after someone read that script; `strictDepBuilds: true` (the
     default only from pnpm 11) makes an unreviewed build fail the install;
     `dangerouslyAllowAllBuilds` is never set. `minimumReleaseAge: 1440` (minutes; since
     pnpm 10.16, where it defaults to 0; setting it explicitly also makes pnpm 11 strict
     about it) keeps versions younger than a day out — the window in which most malicious
     releases are found and pulled; an exception goes in `minimumReleaseAgeExclude` with a
     reason. CI runs `pnpm audit --prod`; an accepted advisory is ignored by its GHSA id with
     a reason. The isolated `node_modules` layout stays (the default `nodeLinker`), so code
     cannot import a package it does not declare; `publicHoistPattern` lists only tooling
     binaries that must resolve from the root (`*eslint*`, `*prettier*`), never runtime
     libraries. Expo supports isolated installs from SDK 54: `nodeLinker: hoisted` only when
     a native library is proven to break without it, scoped to that app (with the default
     shared lockfile the linker applies to the whole workspace, so that app gets its own
     lockfile) and recorded in an ADR naming the library and the error. Why: npm runs every
     dependency's install scripts by default and its flat `node_modules` lets code use
     packages it never declared; a hijacked release does its damage in the hours after it
     is published. Check: `node -p "require('./package.json').packageManager"` prints
     `pnpm@` plus an exact version; `git ls-files pnpm-lock.yaml` → 1 line and
     `git ls-files '*package-lock.json' '*yarn.lock'` → none; Grep the CI workflows for
     `pnpm (i|install)\b` and confirm `--frozen-lockfile` on every hit, and for
     `pnpm audit --prod` → present; Grep `pnpm-workspace.yaml` for `allowBuilds:` and
     `minimumReleaseAge:` → present, and for
     `dangerouslyAllowAllBuilds:\s*true|onlyBuiltDependencies|nodeLinker:\s*hoisted` → none
     (or the ADR exists); Grep `.npmrc` for
     `hoist|node-linker|built-dependencies|release-age` → none; Grep scripts, Dockerfiles, CI
     and docs for `\bnpm (install|i|ci)\b|\byarn (install|add)\b` → none.

G20. **One command to boot and one to test, on any OS.** The root `package.json` scripts
     are the interface every person, agent and doc uses: `docker:up` (build and boot the
     whole stack, migrated and seeded, and wait until it is healthy), `docker:down` (stop,
     keep data), `docker:reset` (wipe volumes and boot again), `test:<scope>` for each
     scope (its lint, format check, typecheck and tests; a Python scope calls `uv run …`),
     `test:web` (the same over every web package), `e2e` and `e2e:install`. Booting differs
     per OS, so `docker:up` runs a node dispatcher (`scripts/up.mjs`) that starts
     `scripts/up.ps1` on Windows and `scripts/up.sh` elsewhere: the boot logic lives once
     per shell, and it creates `.env` from `.env.example` on the first run. E2E base URLs are
     built from the ports in `.env`, never hard-coded. `make` targets, when the repo has a
     Makefile, call these scripts or the same boot script and never repeat their logic. The
     README "Quick start" and "Testing" sections give exactly these commands, each of which
     runs unchanged in PowerShell and in sh (a `pnpm <script>` line, never `export`,
     `./x.sh` or a shell-only chain as the only way). Why: a stack that boots one way on the
     author's machine is a stack a reviewer on Windows cannot run, and two copies of the
     boot logic drift apart. Check:
     `node -p "Object.keys(require('./package.json').scripts).join(' ')"` lists `docker:up`,
     `docker:down`, `docker:reset`, `test:web`, `e2e`, `e2e:install` and a `test:<scope>` per
     scope; Grep the README for `pnpm docker:up` and `pnpm e2e` → present
     under "Quick start" and "Testing"; Grep the E2E config and support files for
     `https?://(localhost|127\.0\.0\.1):\d+` → none.

### Fake-test patterns (`FT`)

The ONE list. Auditors walk it entirely over the specs of a change and cite `FT<n>`. A new
pattern is appended at the end, never renumbered.

FT1. **One-sided boundary.** The limit is tested only on the failing side; the last value
     that PASSES is untested, so `> MAX` mutated to `>= MAX` stays green (also tolerances).

FT2. **Always-true assertion.** `toBeTruthy()` on an object, `toBeLessThanOrEqual(999)`, a
     spy "was called" as the only proof.

FT3. **Generic rejection.** `rejects.toThrow()`, `rejects.toBeTruthy()` or
     `raises(Exception)` that also accepts a TypeError or a 500 — assert the error code
     (`rejects.toMatchObject({ code })`).

FT4. **Invalid base body.** "Rejects X" with a body that was already invalid for another
     reason; one case proves the base body passes and the error is asserted on THAT field.

FT5. **Coinciding fixtures.** Fields or sources share values; every field gets a unique
     value, two tenants get different data, and each switch is flipped one at a time.

FT6. **Doubles that lie.** A database double that ignores the `where`, does not throw on
     what it does not understand, or lacks the unique/partial indexes of the migration;
     data rules are tested against the real database.

FT7. **Clock or timezone dependence.** Time not fixed (fake timers) or timezone not
     pinned (`TZ` explicit, or dates built in local time).

FT8. **Mocks that render `null`.** Every stub renders its `data-testid`.

FT9. **Invalidation by literal key.** Cache invalidation asserted by a literal key or by
     the call instead of its effect — use the exported `*_KEY` and assert the query is
     invalidated with the real key shape (paginated if the query is).

FT10. **Not really mounted.** The test passes without mounting or hydrating; assert what
      the user sees (roles, labels, text), and hydration specs hydrate
      (`renderToString` + `hydrateRoot` with `onRecoverableError`, zero recoverable errors).

FT11. **No empty and no foreign case.** No case for the empty state (null user, empty list,
      absent optional field — one per guard) and, in multi-tenant code, no other-tenant case.

FT12. **Snapshot as the only assertion.** A large snapshot proves the output did not
      change, not that it is right.

FT13. **Rounding-blind fixtures.** Values where round, floor and trunc agree (0.633) or that
      do not move; pick a third decimal ≥ 5 to pin the rounding direction.

FT14. **Unused overrides.** A test helper declares an overrides parameter and never applies it.

FT15. **Orphaned tests.** "Coverage missing on purpose" comments, or tests left without an
      object after a behaviour change; rewrite them for today's behaviour or delete them
      saying why.

FT16. **Upstream normalization.** A `Math.ceil`, clamp, trim or lowercase upstream already
      does what the test believes it verifies.

FT17. **Validation without production metadata.** The validation spec builds its own
      schema or pipe; use the real DTO from its file through the production validation pipe.

FT18. **Invented error shapes.** Errors or states simulated in a shape the library never
      produces; copy the shape from the library's source.

FT19. **Order-dependent checks.** Two checks whose order matters, with no case that
      satisfies both.

FT20. **Allowlist by absence.** An allowlist tested with "does not contain X"; assert the
      EXACT key set.

FT21. **Impossible input.** An input the real component never delivers; test it as it
      arrives — keystroke by keystroke, pasted, prefilled.

FT22. **Fake contract.** A double of a contract nobody meets (a fake analytics global, a
      stubbed toaster); integrations are tested with the REAL library and real providers.

### Definition of done

A task is done when: its acceptance criteria are automated tests that a mutant fails (G2,
G3; in `lite`, tests where cheap); lint, typecheck, tests and build are green for every
touched scope (G10); the audit of the project's rigor has no open BLOCKER or HIGH (G6); no
hard-coded UI copy (G13); the golden path works at the smallest viewport and at desktop
(G10); and the tracking docs are updated in the same commit (G8).
<!-- /kit:block general -->

<!-- kit:block api-common v1 -->
## Rules — API, any backend stack (`A`)

A1. **Feature modules with layers.** Each domain is one module (feature slice) under
    `apps/api`: controller/router = HTTP only (parse, access marker, call the
    service, map to the response DTO, status code); service = business rules and the
    transaction boundary; repository = data access only (returns rows, never raises HTTP
    errors), created when queries are not trivial; DTOs = request and response as SEPARATE
    types. Cross-cutting code lives in `core/` (config, database, errors, http, auth,
    storage, queue, logging) and `core/` never imports a module; a module never imports
    another module's repository. A layer file exists only when the module needs it.
    Why: the folder tree says what the system does and a change stays inside one slice.
    Check: an architecture test in the suite walks imports and fails on `core → module`,
    `controller → repository` and cross-module repository imports; spot-check with Grep
    for `modules/` or `features/` imports under `core/`.
A2. **No SQL in strings.** Queries go through the ORM / query builder or parameterized
    tagged templates. No string concatenation or interpolation into SQL, no unsafe raw
    APIs, no SQL built from user text. Migrations use the migration tool's DDL.
    Why: one interpolated fragment is an injection. Check: lint bans the unsafe APIs;
    Grep `\$queryRawUnsafe|\$executeRawUnsafe|Prisma\.raw\(|sqlalchemy\.text\(|execute\(f["']|(SELECT|INSERT|UPDATE|DELETE) .*\$\{`
    in changed files → none.
A3. **Strict, validated DTOs with shared limits.** Every body, query and param is a DTO:
    unknown keys rejected (strict / forbid extra); body booleans and numbers are NOT
    coerced (`"false"` is not `true`, `""` is not `0`); every string trimmed, minimum
    measured after trim, and capped with the SAME constant as its DB column (one limits
    module); every array has a max length; numbers have min/max; enums validated;
    pagination has a floor and a ceiling (`page ≥ 1`, `1 ≤ limit ≤ 100`) and a stable
    sort from an allowlist. Twin doors reuse the same field definition (G12). Why: the DTO
    is the attack surface, and a cap that differs from the column turns into a 500.
    Check: Grep `z\.string\(\)|@IsString\(\)|: str\b` in DTO files and confirm each hit
    carries its cap (`.max(`, `@MaxLength`, `max_length`); each DTO has a test where the
    base body passes, then the last passing and the first failing value of each cap (FT1,
    FT4).
A4. **No mass assignment; PATCH validates the merged state.** DTO → persistence fields
    are mapped one by one; never spread a request body into a write. A partial update
    loads the stored row, merges (`{ ...stored, ...patch }`), validates the MERGED state
    against the business rules and writes explicit values; "not sent" (`undefined`) keeps
    the stored value, `null` clears it. A new field appears in every create/update mapping
    with a test that reads it back. Prices, totals and ownership are derived server-side,
    never taken from the client. Why: a spread lets clients write columns they must not,
    and validating only the patch lets invalid combinations through. Check: Grep
    `data:\s*\{\s*\.\.\.(dto|body|input|payload)|\*\*(dto|payload|body)\.(model_)?dump\(|Object\.assign\([^,]+,\s*(dto|body)`
    → none.
A5. **Responses are allowlists.** Every response is mapped to a response DTO or an
    explicit field selection: never the raw entity (no password hashes, tokens, internal
    flags, other owners' ids). Private responses are never stored by shared caches
    (`Cache-Control: private, no-store`; never `s-maxage` on private data). A GET never
    changes state. Why: returning the entity leaks every column added later. Check: Grep
    handlers/services returning ORM results without a select/mapper; Grep `s-maxage` on
    authenticated routes; Grep GET handlers that call write methods; integration tests
    assert the EXACT response key set (FT20).
A6. **Money is never float; time has types.** Amounts are fixed-point decimals end to end
    (DB `NUMERIC`/`Decimal(p,s)`, a decimal type in code, a decimal string in JSON) with a
    currency; never float arithmetic; round once, at the end. Instants are `timestamptz`
    (UTC), business dates are `date`; the business time zone comes from configuration.
    Payments, idempotency and money locks follow `addon-money` (`MO`) when it is enabled.
    Why: `0.1 + 0.2` and naive timestamps are silent data corruption. Check: Grep money
    columns typed `Float|Double|REAL|float` and `parseFloat\(|Number\(` / `float\(` on
    amount fields → none.
A7. **One error envelope, one code catalog.** Services raise a domain error with a code
    from ONE catalog; one global handler turns every error (domain, framework, validation,
    DB constraint, unexpected) into the same envelope with at least `code`, `message`,
    `details?` and `requestId`. Business errors carry no free-text message (ids go in
    `details`); clients branch on `code`, never on `message`. No stack traces or internals
    leak; an unexpected error is logged once with its request id and answered as a generic
    500 code (no silent failure: G18). A missing resource, or one that belongs to someone
    else, is 404 (never 403: do not reveal existence). "Not confirmed / pending" is never
    a 2xx. A new code is added to the catalog first, then its copy in every locale, then a
    test asserting the code (`rejects.toMatchObject({ code })` / `exc.value.code ==`, FT3).
    Why: one shape lets every client translate and handle errors; free text cannot be
    localized or matched. Check: Grep framework exceptions thrown outside the handler
    (`new (BadRequest|NotFound|Forbidden|Unauthorized|Conflict)Exception\(`,
    `raise HTTPException\(`) → none; a test proves every catalog code has copy in every
    locale.
A8. **One config module, validated at boot.** The env schema G9 requires is parsed ONCE,
    at startup, by the config module, which is the only reader of `process.env` /
    `os.environ`; services receive typed settings. Production refuses to start on a
    missing secret OR a development default; development shortcuts additionally require a
    non-production environment. Error messages name the keys, never the values. Why: a
    missing secret must stop the deploy, not the first request. Check: Grep
    `process\.env\.|os\.environ|os\.getenv` outside the config module (seeds and scripts
    aside) → none; a test boots with a production env missing a secret and expects a
    non-zero exit.
A9. **Third-party I/O behind a port, with timeouts.** Every external call (HTTP APIs,
    storage, mail, SMS, payments, LLMs) goes through an interface with an adapter; only
    the adapter imports the vendor SDK; every call has a timeout (`AbortSignal.timeout`,
    client timeout, or a `withTimeout` wrapper for clients without signals) and maps
    failures to catalog codes; tests swap the adapter for a fake, never patch internals.
    A URL chosen by a user is never fetched without an allowlist (SSRF). A timeout or 5xx
    is "unknown", not "no": it is never treated as a failure that can be refunded or
    retried blindly. Why: a hung vendor must not hang the API, and a test must not depend
    on the network. Check: Grep `fetch\(|axios\.|httpx\.|requests\.` and vendor SDK imports
    outside adapter folders → none; each adapter call has a timeout.
A10. **Background work off the request.** Anything that can take more than a few seconds
    (AI, imports, exports, bulk e-mail/SMS/push, reports, reminders) runs in a queue or
    background task: the request returns 202 with a status resource. Jobs are idempotent
    (deterministic job id), retried with backoff, and their final failure is reported.
    Queue and cache connections are lazy (never opened at import time), with no reconnect
    loops and no polling without a stop condition. Why: proxies cut long requests and the
    client retries, doing the work twice; eager connections burn paid resources. Check:
    Grep `new (Redis|Queue|Worker)\(` and confirm none runs at module top level; Grep
    `setInterval\(` and confirm each has a stop condition; job ids are derived from the
    payload.
A11. **Hand-written migrations with constraints and indexes.** Migrations are reviewed
    DDL written or edited by hand, never applied by an interactive/auto tool against a
    shared database (`prisma migrate dev`, `prisma db push`, unreviewed autogenerate).
    They are additive (expand → backfill verified → contract), carry their constraints
    (`NOT NULL`, `CHECK`, `UNIQUE` incl. partial, FK with an explicit `ON DELETE`) and an
    index for every column used in a filter, sort or join. Once applied anywhere shared, a
    migration is never edited: changes are new migrations, timestamped with the real time
    and ordered after the last one. A drift test proves migrations build exactly the
    declared schema. Why: auto tools propose dropping what only lives in SQL (partial
    indexes, CHECKs). Check: Grep docs, scripts and package files for `migrate dev|db push`
    → none; the drift test passes; every new filter/sort column has an index in the
    migration.
A12. **HTTP security baseline.** Security headers (helmet or equivalent); JSON body
    ≤ 1 MB (uploads have their own limits); CORS from an env allowlist (never `*` with
    credentials) with every custom header listed; a global rate limit plus stricter limits
    on auth, OTP, password reset, public forms and every endpoint that costs money
    (paid third parties, presigned uploads, e-mail/SMS); passwords hashed with Argon2id
    (or bcrypt); tokens never in URLs (except hashed single-use links); `JSON.parse` of
    external input in try/catch and validated after; no regex built from user text;
    objects from users never merged without filtering `__proto__`/`constructor`; webhooks
    verify the signature and process only what it covers; no PII, tokens or secrets in
    logs, analytics or error reports. Why: these are the holes every audit finds first.
    Check: Grep the bootstrap for headers, body limit, CORS and throttler; Grep
    `new RegExp\(`, `JSON\.parse\(`, `[?&](token|access_token|id_token)=` in changed files.
A13. **Uploads are capped and verified.** Files go to private storage through the storage
    port: the server builds the key (`<owner>/<kind>/<uuid>`), enforces a MIME allowlist
    (never SVG or HTML), a size cap in the signed policy or the multipart limits (plus
    file count and text-field caps), and sniffs the real type from magic bytes after
    upload; downloads use short-lived signed URLs (≤ 5 min) with `attachment`. A file or
    image URL sent by a client is accepted only if it points to our storage and belongs to
    that owner and resource; the server re-derives the key. Why: upload endpoints are the
    cheapest path to stored XSS and storage abuse. Check: Grep upload handlers for the
    MIME allowlist, size limits and magic-byte check; an integration test uploads a
    renamed SVG and an oversized file → rejected with their codes.
A14. **Access is explicit and default-deny.** Every route declares exactly one access
    marker (public, authenticated, role/permission, …); a route without one is denied at
    runtime AND fails a route-discovery test in CI. Ownership is checked in the service
    query (`where owner = …`), not after loading. Why: a forgotten decorator must break
    the build, not expose data. Check: the route-discovery test lists 0 unmarked and 0
    double-marked routes; Grep new public markers and justify each.
A15. **Structured logs with a request id.** Logs are structured (JSON in containers);
    every request gets one id (accepted from `X-Request-ID` or generated), returned in the
    `X-Request-ID` response header and in every error envelope, and propagated to jobs it
    enqueues; logs carry ids and enums, never PII, tokens or secrets (a redaction list
    covers auth headers, cookies, passwords). Why: one id joins the user's report, the log
    and the trace. Check: Grep `console\.log|print\(` in source → none; an integration
    test asserts the header and the envelope share the id.
A16. **Strict typing.** TypeScript `strict: true` with lint `no-explicit-any`, or Python
    `mypy --strict`; `any` / `Any` only at true boundaries with a comment saying why; no
    `@ts-ignore` / `type: ignore` without a reason. Why: types are the cheapest test.
    Check: `go build ./...` exits 0; Grep `as any|: any\b|@ts-ignore|# type: ignore$`
    in changed files.
A17. **Unit and integration tests, errors by code.** Services are unit-tested with
    hand-written fakes; data rules (filters, uniqueness, constraints, locks, ownership)
    are tested against a real database (container or throwaway database), never a DB
    double (FT6). Each endpoint's integration test covers the happy path, a validation
    failure with a base body that passes first (FT4), an access denial and a 404 for a
    missing or foreign resource. Errors are asserted by `code` (FT3). Why: a double that
    ignores `where` passes every wrong query. Check: every new route appears in an
    integration spec; Grep tests asserting `message` text → none.
A18. **Backward-compatible contract.** Contract changes are additive or versioned. A
    hardened endpoint keeps accepting (and ignoring, with a cap) what published clients
    still send, and answers with codes old clients already handle; a new required request
    field ships API-first. When a change affects the contract, the commit body states the
    deploy order (API before clients, G7). A generated API spec or client, when the
    project has one, is regenerated and committed in the same change. Why: clients and
    API are never deployed at the same instant. Check: diff of the API spec; commit body
    names the deploy order when request/response shapes changed.
A19. **Tooling green with a coverage gate.** `go vet ./...`,
    `go build ./...`, `go test -race ./...`, the integration suite and
    `go build ./...` exit 0 (G10). The G2 coverage gate covers the API's core and
    modules (generated code, module wiring and DTO declarations excluded), enforced by the
    runner config so CI fails below it. Why: a gate in config cannot be
    forgotten. Check: run each command and read its exit code; the thresholds live in the
    runner config.

A20. **A third party with no SLA on a critical path is hedged.** When a request a user is
    waiting on depends on a provider without an SLA (a public geocoder, a free search or
    lookup API), its A9 adapter is a hedge behind the same port: the primary searches
    alone for a short delay (from configuration); if it is late, fails or answers empty, a
    backup provider is asked as well. The first answer WITH results wins; the loser is
    cancelled AND awaited, so no task, connection or log line outlives the request; the
    whole search keeps ONE overall deadline. Only the primary's own non-empty answers are
    cached: a backup's answer or an empty answer may come from an outage and would be served
    for the whole TTL after the provider recovers. Both answering empty (or the primary
    empty and the backup failing) is "no results"; the primary failing with the backup
    empty, both failing, or the deadline passing is the catalog's `unavailable` code (A7),
    never "no results". The client shows that state with a retry and never silently
    degrades to unvalidated free text (W18). Why: waiting for the primary's timeout before
    asking a backup makes every slow request slower, and an outage reported or cached as
    "nothing found" lies to users long after it ends. Check: the hedge's tests cover a slow
    primary (the backup's answer wins), an empty primary, a failing primary with an empty
    backup (→ `unavailable`), both failing, the deadline, and the loser's cancellation with
    no task left pending afterwards (Python: `asyncio.all_tasks()` holds only the test's
    own; Node: the loser's `AbortSignal` fired); the cache test proves a backup answer and
    an empty answer are not stored.
<!-- /kit:block api-common -->

<!-- kit:block api-go v1 -->
## Rules — Go API: chi + oapi-codegen + pgx/sqlc + River (`AG`)

The stack-agnostic API rules are `A1…A20` and, with the money addon, `MO1…MO6`; these say
how each one is done in Go, for every scope of kind api with stack `go`: net/http with chi
v5, a contract-first OpenAPI 3 spec served by an oapi-codegen strict server, Postgres
through a pgx v5 pool and sqlc, goose migrations, River jobs, log/slog and the standard
`testing` package. Commands run from the scope's path (the folder with `go.mod`).

AG1. **Layout: `cmd/`, features under `internal/`, `platform` never imports a feature.**
    `apps/api` follows (A1 in Go terms):
    ```
    go.mod  go.sum  sqlc.yaml  .golangci.yml  .testcoverage.yml
    api/openapi.yaml  api/oapi-codegen.yaml  api/spec.go (embeds the spec)
    cmd/<app>/main.go             api, worker, migrate, mock<provider>: wiring only
    migrations/<YYYYMMDDHHMMSS>_<what>.sql   (+ embed.go)
    internal/api/                 generated strict server and models (api.gen.go)
    internal/app/                 composition: the strict Server, AccessRules(), router
    internal/platform/            config/ db/ httpx/ apperr/ log/ auth/ money/ clock/ realtime/ …
    internal/<feature>/
      handler*.go  service*.go  model*.go  port*.go  jobs*.go  errors.go  doc.go  *_test.go
      store/                      queries.sql + sqlc output (+ hand-written query helpers)
      provider/<vendor>/  provider/fake/   adapters of the feature's port and its fake
    ```
    `handler*.go` is HTTP only (generated request → domain input → service → generated
    response) and never imports `store` or pgx; `service*.go` owns rules and the
    transaction and never imports `net/http` or `internal/api`; `store/` is data access only
    and never imports `net/http`, `internal/api` or `httpx`. A feature never imports another
    feature's `store/` (it calls that feature's service); `internal/platform` never imports a
    feature or `internal/app`. No package-level mutable state (`var x = …` that code writes
    or that holds a client, pool or config; sentinel errors, compiled regexps and
    `embed.FS` are fine) and no `init()` with side effects: `main` builds every dependency
    and passes it down. Why: the tree says what the API does, and explicit wiring is what
    lets a test swap any dependency. Check: `/kit:audit quick` (layout, layer imports,
    cross-feature `store` imports, `init()`, globals); Grep `^func init\(\)` and
    `^var\s+[a-z]\w*\s*=` in non-test files → each one justified.

AG2. **Contract first: OpenAPI 3 + oapi-codegen strict server on chi.** `api/openapi.yaml`
    is the contract (A18): a route, field, status or error response is added to the spec
    FIRST, then `go generate ./internal/api` (oapi-codegen v2 with `models`, `chi-server`
    and `strict-server`, run as `go tool oapi-codegen`) regenerates `internal/api/api.gen.go`,
    and only then the handler is written. Every operation has an `operationId`; the
    composition `Server` in `internal/app` embeds one handler struct per feature and asserts
    `var _ api.StrictServerInterface = (*Server)(nil)`, so a missing operation is a compile
    error. Routes that need the raw request (signed webhooks, SSE) stay in the spec and are
    left out of generation with `output-options.exclude-operation-ids`, then mounted by
    hand. No other router or framework (gin, echo, fiber, gorilla/mux). Handlers return
    generated response types built field by field (A5); sqlc rows never reach JSON
    (`emit_json_tags: false`). Why: one spec generates the server, the clients and the
    validator, so they cannot drift. Check: `go generate ./... && git diff --exit-code`
    → no diff; Grep `gin-gonic|labstack/echo|gofiber|gorilla/mux` in `go.mod` → none.

AG3. **Validation from the spec, caps in the spec.** The kin-openapi request validator
    (`github.com/oapi-codegen/nethttp-middleware`, `OapiRequestValidatorWithOptions` with
    `ErrorHandlerWithOpts` mapped to the A7 envelope and `NoopAuthenticationFunc`, since
    AG15 owns security) runs before the strict server, loaded from the spec file itself
    (`api/spec.go` embeds `openapi.yaml`; the generated embedded copy drops excluded
    operations). Every request object schema has `additionalProperties: false`; every
    string a `maxLength` equal to its column's CHECK, every array `maxItems`, every integer
    `minimum`/`maximum`, enums as `enum`; amounts are decimal strings with a `pattern`, never
    `number` (A3, A6). Validation errors name the field (`details.field`) and never echo the
    value. Business rules the schema cannot express are checked in the service. Why: the
    validator only rejects what the spec declares, and an uncapped string is a 500 at the
    database. Check: a spec test walks every request schema and fails on an object without
    `additionalProperties: false`, a string without `maxLength` or an array without
    `maxItems`; the router test sends an unknown field, a float amount and an over-long
    string → `validation.failed`.

AG4. **One error type, one writer, errors wrapped.** Services return `*apperr.Error`
    (`apperr.New(apperr.CodeX, details)` / `apperr.Wrap(code, cause, details)`) with the
    code from ONE catalog (`internal/platform/apperr`: the code, its HTTP status and its
    message key; locale copy per code, A7). `httpx.WriteError` is the only place that turns
    an error into a response: the strict server's `RequestErrorHandlerFunc` and
    `ResponseErrorHandlerFunc`, the validator hook and hand-mounted handlers all call it;
    an unknown error is logged once with the request id and answered as `internal.error`.
    Errors are wrapped with `fmt.Errorf("…: %w", err)` and inspected with `errors.Is` /
    `errors.As`, never `==` against a wrapped error or `err.Error()` text. No `panic` on a
    request path; the `Recover` middleware turns a panic into a logged 500 envelope and
    re-panics `http.ErrAbortHandler`. Why: one place decides status, code and redaction.
    Check: Grep `http\.Error\(` outside `internal/platform/httpx` → none; Grep `\bpanic\(`
    under `internal/` outside tests → each one justified; golangci-lint `errorlint` on; a
    test proves every `apperr.All()` code has a status and copy in every locale.

AG5. **Server hardening and graceful shutdown.** The `http.Server` sets
    `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` and `MaxHeaderBytes`
    (a stream lifts its own write deadline with `http.ResponseController`). The middleware
    order is fixed: request id → recover → access log → security headers → CORS (origins
    from config, every custom header listed) → client IP (`middleware.ClientIPFrom*`, the
    trusted proxies from config) → rate limit (`httprate.LimitBy` on the canonical client
    IP; stricter per route for auth, OTP and money) → body limit (`http.MaxBytesReader`,
    1 MiB by default, A12) → guard (AG15) → per-request timeout →
    validator. `main` runs on `signal.NotifyContext(…, SIGINT, SIGTERM)` and an `errgroup`;
    on cancel it calls `srv.Shutdown` with a deadline from `context.WithoutCancel`, stops
    River (`client.Stop`), closes streams and finally the pool. Why: a zero timeout is a
    slowloris door, and a deploy must not cut requests or jobs in half. Check: Grep
    `http\.Server\{` and confirm the five fields; Grep `ListenAndServe\(` outside
    `cmd/` → none; Grep `cmd/*/main.go` for `NotifyContext` and `Shutdown(` → present.

AG6. **`context.Context` everywhere I/O happens.** Every function that does I/O or may block
    takes `ctx context.Context` as its first parameter and passes it on; outbound requests
    use `http.NewRequestWithContext`; queries take the request's ctx, so the per-request
    deadline cancels them. `context.Background()` / `context.TODO()` appear only in `main`
    and tests (`t.Context()`); work that must outlive the request uses
    `context.WithoutCancel(ctx)` plus its own timeout, or a River job. Values in a context
    are only request-scoped data with unexported key types (request id, principal). Why: a
    lost ctx is a query that keeps running after the client left and a shutdown that never
    ends. Check: Grep `context\.(Background|TODO)\(\)` under `internal/` outside `_test.go`
    → none; golangci-lint `contextcheck` and `noctx` on.

AG7. **Third parties behind a port, with a fake and a mock server.** Each provider (bank or
    PSP, KYC, mail, SMS, storage) is an interface (`port.go`) in the feature that uses it,
    with ctx-first methods and domain types; the HTTP adapter lives in
    `provider/<vendor>/` and is the only code that knows the vendor's URLs, headers and
    JSON. `provider/fake/` implements the same port in memory with the provider's real
    semantics (idempotent by our reference, async completion, scriptable failures) and
    backs a standalone mock server of the vendor's wire format (`cmd/mock<vendor>`), used
    for local dev and for the adapter's contract test (`httptest` against it). Tests and
    local dev never call the real vendor; tests that do carry the `live` build tag. A
    timeout, network error or 5xx is `ErrUncertain`, never "failed" (A9, MO4). Why: the
    feature is tested and demoed without the vendor, and the adapter is proven against
    the same format. Check: Grep `net/http` imports under `internal/<feature>/` outside
    `provider/` and `handler*.go` → none; each port has a `provider/fake` implementing it
    (`var _ Port = (*fake.Provider)(nil)`) and an adapter contract test.

AG8. **Outbound HTTP: one client per adapter, every timeout set.** The adapter builds its
    own `*http.Client` with `Timeout` and a `Transport` with dial, TLS-handshake and
    response-header timeouts; never `http.DefaultClient`, `http.Get/Post/Head/PostForm` or
    `&http.Client{}`, never `InsecureSkipVerify`. Every response body is closed and read
    through `io.LimitReader`. Retries only for network errors, 429 and 5xx, with jittered
    backoff inside the caller's ctx deadline, and a POST is retried only with the
    provider's idempotency key set to our reference. Why: `http.DefaultClient` has no
    timeout, and a hung vendor must not hang the API (A9). Check: Grep
    `http\.DefaultClient|http\.(Get|Post|Head|PostForm)\(|http\.Client\{\s*\}|InsecureSkipVerify:\s*true`
    → none; Grep `http\.Client\{` outside `provider/` and `internal/platform/` → none;
    golangci-lint `bodyclose` and `noctx` on.

AG9. **Postgres through pgx v5 and sqlc; no ORM, no SQL strings.** One `*pgxpool.Pool`
    built in `main` (`db.NewPool`, with `statement_timeout`,
    `idle_in_transaction_session_timeout` and `timezone=UTC` runtime params) and passed
    down. Queries live in `internal/<feature>/store/queries.sql` and are generated by sqlc
    (`sql_package: pgx/v5`, `emit_json_tags: false`, uuid and timestamptz overrides) with
    `schema: migrations`, so sqlc reads the real migrations. Generated code is committed and
    never edited. No ORM or query builder (gorm, ent, bun, sqlx, sqlboiler) and no
    `lib/pq`; no SQL built with `fmt.Sprintf` or `+`; raw `Query/Exec` with a literal only
    in `internal/platform` (LISTEN, test database setup). Lists page with a limit from the
    A3 ceiling and an `ORDER BY` with a unique tiebreaker. Why: A2 by construction, and
    every query is type-checked against the schema at generate time. Check:
    `sqlc diff` (or `sqlc generate && git diff --exit-code`) → no diff; `sqlc vet` passes;
    Grep `fmt\.Sprintf\(\s*.\s*(SELECT|INSERT|UPDATE|DELETE|WITH)\b` (case-insensitive)
    → none; Grep `gorm\.io|entgo\.io|uptrace/bun|jmoiron/sqlx|lib/pq` in `go.mod` → none.

AG10. **goose migrations by hand, with a drift test.** Migrations are SQL files
    `migrations/<YYYYMMDDHHMMSS>_<what>.sql` with `-- +goose Up` / `-- +goose Down`
    (functions inside `-- +goose StatementBegin/End`), a first-line comment saying why,
    embedded with `//go:embed *.sql` and applied by `db.Migrate` (goose v3 `NewProvider` on
    `stdlib.OpenDBFromPool`) from `cmd/migrate` and the tests: never by hand against a
    shared database, never edited once applied (A11). They carry every constraint: `NOT
    NULL`, `CHECK` (lengths equal to the spec's caps, amounts, enums as `CHECK … IN`),
    `UNIQUE` incl. partial, FKs with explicit `ON DELETE`, an index per filter, sort or join
    column. River's tables come from `river migrate-get` saved as a goose migration, so
    one tool owns the schema. The drift test migrates an empty database up, all the way
    down (`DownTo(ctx, 0)`) and up again; with sqlc reading the same files, generated code
    and database cannot disagree. Why: a migration that only works one way is found in
    production, and only constraints in the schema are enforced. Check: the drift test
    passes; `ls migrations/` names are ordered timestamps; every new filter or sort column
    has an index in its migration.

AG11. **Transactions through ONE helper; locks and serializable retry.** Every multi-statement
    write runs in `db.WithTx(ctx, pool, opts, func(tx pgx.Tx) error)` (built on
    `pgx.BeginTxFunc`: commit on nil, rollback on error or panic), called from the service,
    with `store.New(tx)` inside; what decides is read INSIDE the transaction; no network
    call inside it (call the provider before, re-check after on the locked row); no
    goroutine shares a `tx`. Contended rows are locked with `SELECT … FOR UPDATE` as the
    first statement, several rows in ONE global order (`ORDER BY id`), or changed with a
    conditional `UPDATE … WHERE <condition>` declared `:execrows` whose row count is
    checked (0 → the catalog code, never silently ignored). Money and other invariants under
    contention run in `db.WithSerializable`, which retries SQLSTATE `40001`/`40P01` a bounded
    number of times with jittered backoff and then returns `operation.in_progress` (409):
    this is MO3 in Go. Why: check-then-write without a lock lets two requests pass the same
    check. Check: Grep `\.Begin(Tx)?\(` outside `internal/platform/db` → none; Grep
    `:execrows` queries and confirm every caller checks the count; a concurrency test fires
    N goroutines at one row and asserts exactly the allowed number of effects.

AG12. **Money is `int64` minor units with its currency; never `float64`.** On top of A6:
    amounts are `money.Amount{Minor int64, Currency}` in code and `bigint` columns with a
    `char(3)` ISO 4217 currency (CHECK `^[A-Z]{3}$`) on EVERY row that holds an amount;
    the contract carries a decimal string plus currency, converted exactly at the edge by
    `money.ParseDecimal` / `Amount.Decimal` with the currency's exponent (more fraction
    digits than the currency has is a validation error, never a rounding); `Add` refuses a
    currency mismatch and int64 overflow; splits go through ONE largest-remainder helper
    `money.Split` (MO1). `float32`/`float64`, `math.Round` and `strconv.ParseFloat` never
    touch an amount. Why: `4.35 * 100` is `434.99999999999994`, and a sum across
    currencies is meaningless. Check: Grep `float(32|64)` and `ParseFloat` on lines naming
    an amount, balance, price, fee, total or cents, and anywhere under `**/money/**` →
    none; the money tests pin rounding direction and the round trip of `"0.12"` (FT13).

AG13. **Jobs and outbox with River; no fire-and-forget goroutines.** Background work (A10)
    is a River job (`river` with `riverpgxv5`): args types with `Kind()` and `InsertOpts()`
    (`MaxAttempts`, `UniqueOpts{ByArgs: true}` so a retried transaction cannot enqueue
    twice), inserted with `InsertTx` in the SAME transaction as the effect it follows
    (transactional outbox: the job exists if and only if the effect committed). Workers
    are idempotent (River delivers at least once), set a `Timeout`, live in the feature's
    `jobs*.go` and run in `cmd/worker`; the API process holds an insert-only client.
    Periodic work is a `river.PeriodicJob` (run on the elected leader only, so N replicas
    enqueue it once). A handler, service or store never starts a `go` statement; bounded
    fan-out inside a request uses `errgroup.WithContext` with `SetLimit`. Why: a goroutine
    started from a request dies with the pod and nobody retries it. Check: Grep
    `^\s*go\s+(func\b|[\w.]+\()` in `handler*.go`, `service*.go` and `store/` → none;
    Grep `\.Insert\(` (not `InsertTx`) in services next to a write → each one justified.

AG14. **Configuration parsed once into a typed struct.** `internal/platform/config` is the
    only reader of the environment (A8): `config.Load(environ)` parses every variable with
    `github.com/caarlos0/env/v11` (`required`, `notEmpty`, defaults) into one `Config`,
    validates it (production refuses development values: `*` CORS, http or localhost
    provider URLs) and returns an error naming the keys, never the values; `main` exits
    non-zero on it before opening any port. Secrets are `config.Secret` (its `String`,
    `GoString`, `LogValue` and `MarshalText` print `[REDACTED]`; `Reveal()` is the only
    way to read it). Why: a missing secret must stop the boot, not the first request.
    Check: Grep `os\.(Getenv|LookupEnv|Environ)\(` outside `internal/platform/config` → none;
    a test loads a production environment with a development value and expects an error.

AG15. **Auth: JWT from a JWKS, default-deny by operationId.** Bearer tokens are verified
    with `github.com/golang-jwt/jwt/v5` and keys from the identity provider's JWKS
    (`github.com/MicahParks/keyfunc/v3`, refreshed in the background): `WithValidMethods`
    (RS256/ES256 only), `WithIssuer`, `WithAudience`, `WithExpirationRequired`,
    `WithIssuedAt`, a small leeway. `internal/app.AccessRules()` maps EVERY operationId of
    the spec to exactly one rule (`Public`, `Signed`, `Authenticated`, `Role(r)`, optionally
    `.WithStepUp(maxAge)` for money movements, approvals and payout-destination changes:
    MFA in `amr` and `auth_time` recent). One middleware finds the request's spec operation
    (kin-openapi router) BEFORE any handler or validator runs and applies its rule; an
    operation without a rule, or a path outside the spec, is denied. The route-marker test
    (A14) walks the spec: every operation has a rule, no rule names an unknown operation,
    and `security: []` is declared exactly on the operations whose rule needs no token.
    Ownership is part of the query (`WHERE id = $1 AND owner_id = $2`), never checked
    after loading; foreign → 404. Why: a forgotten marker must break the build, not expose
    an endpoint. Check: `TestAccessRulesCoverTheSpec` passes; Grep `auth\.Public\(\)` in
    the diff and justify each hit.

AG16. **Signed webhooks: raw body, constant time, stored then processed.** A webhook route is
    hand-mounted (AG2) under the `Signed` rule: it reads the raw body (already capped),
    verifies the provider's signature (HMAC-SHA256 over the exact bytes, plus a timestamp
    within a tolerance window against replays) with `hmac.Equal`, then decodes; it stores
    the event in a table keyed by `(provider, event_id)` (a redelivery inserts nothing) and
    enqueues the processing job with `InsertTx` in the same transaction, then answers 2xx.
    The job uses only the reference from the event and re-reads the state at the provider
    (MO4); amounts or statuses in a webhook body are never trusted. Why: a forged or
    replayed webhook is the cheapest way to mint money. Check: Grep `==` or `bytes\.Equal`
    on a signature, MAC or digest → none (`hmac.Equal` only); a test sends a valid
    signature, a body changed after signing, another secret, the last accepted and first
    rejected timestamp age, and a redelivery (one job).

AG17. **slog JSON logs with the request id; no PII.** `log/slog` with `slog.NewJSONHandler`
    is configured once in `main` (`internal/platform/log.New`, then `slog.SetDefault`): a
    handler adds `request_id` from the context to every line, and `ReplaceAttr` redacts
    authorization, cookies, passwords, tokens, secrets, account numbers (CVU/CBU) and
    document numbers whatever the caller passes; types that hold personal data implement
    `slog.LogValuer`. Code logs with `slog.InfoContext(ctx, …)` / `ErrorContext`; no
    `fmt.Print*`, `log.Print*`/`log.Fatal*`, `println`, logrus, zap or zerolog. The request
    id (A15) is read from a safe `X-Request-ID` or generated, returned in the header and the
    error envelope, and put in the args of every job a request enqueues. With the
    observability addon, OpenTelemetry hooks (`otelhttp`, a pgx tracer, River's otel
    middleware) live in `internal/platform/observability` and are no-ops without config
    (OB1, OB2). Why: one id joins the user's report, the log line and the trace. Check:
    Grep `fmt\.Print|log\.(Print|Fatal|Panic)|^\s*println\(` outside tests → none (also
    golangci-lint `forbidigo`); the router test asserts the header and the envelope carry
    the same id.

AG18. **Tests: stdlib, tables, real Postgres, race detector, coverage gate.** Runner
    `go test` (`go test`). Tests use the standard `testing` package with
    table-driven cases and `t.Run`, `t.Parallel()` where they share nothing, `t.Context()`,
    and hand-written fakes behind the ports (no mock generators; A17). Services are tested
    against real Postgres: `testcontainers-go` `modules/postgres` started once per test
    binary in `TestMain`, a template database migrated with the REAL migrations, one cloned
    database per test (`CREATE DATABASE … TEMPLATE`); these files carry
    `//go:build integration`. Handlers are tested through the whole router with
    `httptest` (errors asserted by `code`, exact key sets, FT3, FT20). Time comes from an
    injected `clock.Clock` (FT7); no `time.Sleep` in tests (poll with a deadline or use the
    fake clock). Every package is run with `-race`. The G2 gate: `go test -race
    -tags=integration -coverpkg=./internal/... -coverprofile=cover.out ./...` then
    `go tool go-test-coverage --config=.testcoverage.yml` with `threshold.total:
    70`, excluding generated code (`*.gen.go`, `store/`) and `cmd/`. Why: a
    double cannot prove a `WHERE`, a lock or a constraint, and data races hide in
    concurrent money code. Check: `go test -race ./...` and `go test -race -tags=integration ./...`
    exit 0 with Docker up; Grep `time\.Sleep\(` in `_test.go` → none;
    `.testcoverage.yml` has the threshold.

AG19. **Tooling, pinned and green.** `go.mod` pins the `go` version (and `toolchain` when the
    team needs one) and every dependency; `go.sum` is committed; tools are `tool` directives
    in `go.mod` (oapi-codegen, goose, go-test-coverage, govulncheck) run with `go tool …`,
    while sqlc and golangci-lint run at a pinned version (binary or image). `.golangci.yml`
    (`version: "2"`) enables at least the standard set plus `bodyclose`, `contextcheck`,
    `errorlint`, `forbidigo` (the calls banned by AG6, AG8, AG14, AG17), `gosec`, `noctx`,
    `rowserrcheck`, `sqlclosecheck`, and the `gofumpt` and `goimports` formatters. CI runs
    `go mod tidy && git diff --exit-code go.mod go.sum`, `go mod verify`,
    `go vet ./...`, `go build ./...`, the tests with the gate,
    `go build ./...` and `go tool govulncheck ./...` (A19). Why: an unpinned
    toolchain or a skipped linter is a build that passes on one machine only. Check: run
    each command and read its exit code; Grep `.golangci.yml` for the linters above.

AG20. **Mutation check with `go test`.** G3 for this scope uses the shared recipe's
    copy-and-restore mode (runner-agnostic: backup, write the mutant, run, restore in a
    `finally`, verify the sha256), one mutant per run, on a green package. From the repo
    root:
    `node .claude/kit/mutant/mutant.mjs --mode copy --cwd <scope path> --file internal/<feature>/service.go --from '<snippet>' --to '<mutant>' -- go test -count=1 ./internal/<feature>/...`
    (add `-tags=integration` when the killing test is an integration test). `-count=1`
    defeats the test cache; no `-cover`. KILLED needs a `--- FAIL: TestX` line; a mutant
    that does not compile or fails `go vet` (`[build failed]`, an unused variable) is
    INVALID, never killed. Run it only while no `air`/watcher or other agent reads the file
    (G3); `auditor-tests` reasons the mutant and re-runs nothing on disk. Why: there is no
    in-memory transform for Go, so the file itself must change, and only a verified restore
    keeps that safe. Check: the builder report's mutation table lists each command with
    `KILLED` and `sha256 unchanged`; `git status` shows no source change after the run.
<!-- /kit:block api-go -->


<!-- kit:block mobile-common v1 -->
## Rules — Mobile (`M`)

M1. **Feature structure.** Same feature layout as the web front-end. Each feature lives in
    `features/<feature>/` under `apps/mobile`: ONE root component named after the entity (`Orders.tsx`, not
    `OrdersList.tsx`), `components/` (dumb views, one component per file), `hooks/` (all
    logic and decisions), `mutations/`, `services/` (one function per endpoint, URL taken
    from the endpoint catalog), `types.ts`, `data.ts`. Screen/route files only render a
    feature root. Queries shared by features live in one shared queries folder and export
    their `*_KEY`; a feature never imports another feature (shared code moves to the app's
    shared folders). A condition that decides an amount, availability or whether an action
    is enabled is a named function in `hooks/` or `helpers/` with a test, never inline JSX.
    Why: logic in hooks and helpers runs in node tests; logic in views needs a device.
    Check: Grep `useEffect|useQuery|useMutation|fetch\(|axios|toast|Alert\.alert` in
    `features/*/components/` → 0; Grep `use(State|Effect|Query|Mutation)\(` in route/screen
    files → 0; Grep `features/` in each feature's imports → only its own folder.
M2. **Server state and user feedback.** Server data is read only through TanStack Query
    hooks from the shared queries folder; writes go through `useMutation` in `mutations/`.
    Every mutation has `onSuccess` AND `onError`; the user feedback (toast, snackbar,
    `Alert`) is emitted there and nowhere else (sole exception: the validation gate of a
    hook's `submit()`); it invalidates, with the exported `*_KEY` constants, every query
    whose data changed; and its error branch maps the API error `code` to i18n copy, never
    showing the raw server message. A notice that must be seen when a screen opens is
    rendered IN the screen, not as a toast (the toast host mounts later and drops it). A
    dialog or sheet closes only in `onSuccess`, never in `onSettled`. Why: scattered
    feedback gets lost or duplicated, and a write without invalidation leaves stale screens.
    Check: Grep `useMutation\(` outside `mutations/` → 0; Grep count of `onSuccess` and
    `onError` per file in `mutations/` → ≥ 1 each; Grep `toast|Toast\.show|Alert\.alert` in
    `components/` and `hooks/` → 0 or the submit gate; Grep (multiline)
    `onSettled[\s\S]{0,200}(close|dismiss|setOpen\(false\))` → 0.
M3. **Effects, polling and app lifecycle.** `useEffect` is the last resort and never lives
    in `components/`: derive in render, act in the event handler, read with a query
    (`enabled`, `staleTime`), write with a mutation. No network call inside an effect.
    Every poll (`refetchInterval`, `setInterval`) has a stop condition and never starts a
    tick while the previous one runs. TanStack Query is wired to the app lifecycle ONCE at
    the root: `focusManager` to `AppState` and `onlineManager` to the network state,
    because React Native has no window focus/online events and screens stay mounted for
    hours. Why: an effect with unstable dependencies refetches forever from an idle screen,
    and without focus wiring data stays stale until a cold start. Check: Grep (multiline)
    `useEffect\([\s\S]{0,800}?(fetch\(|\.get\(|\.post\(|mutate\(|mutateAsync\(|refetch\(|invalidateQueries\()`
    → 0; each `refetchInterval|setInterval\(` hit has its stop condition; Grep
    `focusManager\.|onlineManager\.` in the root providers → both present.
M4. **Secrets only in the OS secure store.** Refresh tokens, credentials and any secret
    persist only in the OS secure store (Keychain / Keystore); short-lived access tokens
    live in memory; logout deletes every secure entry and the in-memory session. Never
    AsyncStorage, MMKV, SQLite, plain files or a persisted state store for secrets. A
    secure-store read error (locked device, corrupted keystore) means "no session", never a
    crash. No token ever travels in a URL. Why: plain storage is readable from backups and
    rooted devices, and a token left after logout signs the next person in. Check: Grep -i
    `(AsyncStorage|MMKV|storage)\.(setItem|set)\([^)]*(token|secret|password|credential)` →
    0; persisted stores hold no token fields; a test asserts logout deletes the secure
    entry; Grep `[?&](token|access_token|refresh_token|id_token)=` → 0.
M5. **Only public config in the bundle.** Everything shipped in the app bundle is public:
    only variables the framework marks public get in (Expo: see `ME3`), never a secret even
    with that prefix, and keys of paid or privileged services stay on the API. All
    environment reads live in ONE config module that features import. Why: anyone can
    unpack the bundle and read every string in it. Check: Grep
    `process\.env|Constants\.expoConfig\?\.extra` outside the config module → 0; Grep -i
    `SECRET|PRIVATE|PASSWORD` in the mobile env files and build profiles → 0.
M6. **Loading, empty, error and offline states.** Every screen that shows server data
    renders four states: a skeleton while loading (gated on `isLoading`, not `isPending`,
    so a disabled query never shows an eternal skeleton), an empty state with a call to
    action, an error state with a retry, and an offline banner when the device has no
    connection. Loading or failed data is never rendered as a value (no fake `0`, no
    editable controls without data). A send button is disabled while its mutation is
    pending, and the HTTP client sets a request timeout. Why: slow and offline are normal on
    phones, and a blank or endless spinner reads as broken. Check: Grep `\.isPending` on
    queries that set `enabled:` → 0; the screen's hook spec asserts the four branches; Grep
    `timeout` in the HTTP client config → present.
M7. **Accessibility.** Every interactive element has an accessible name and role
    (`accessibilityLabel` + `accessibilityRole`, or `aria-label` + `role`), icon-only
    buttons included; selected/disabled/checked state is exposed (`accessibilityState`) and
    never conveyed by colour alone. Touch targets are at least 44×44 pt (extend small icons
    with `hitSlop`). Text follows the system font size: no `allowFontScaling={false}`, no
    fixed-height text containers, and screens survive the largest accessibility size
    without clipping (`maxFontSizeMultiplier` only with a comment saying why). Contrast
    meets WCAG AA (4.5:1 text, 3:1 large text and UI parts) in light and dark. Why: these
    are the users who cannot work around a broken control. Check: Grep
    `allowFontScaling=\{false\}` → 0; every `<(Pressable|TouchableOpacity|TouchableHighlight)`
    hit has a role and a label or text child; the builder report records a pass with the
    largest text size and a screen reader (VoiceOver / TalkBack) on the changed screens.
M8. **i18n.** Every user-visible string (alerts, toasts, accessibility labels, notification
    text and error copy included) comes from the i18n catalog through `t('namespace.key')`;
    the source locale is `es-AR` and every key exists in every shipped locale.
    Dates, numbers and currency use `Intl` formatters with the active locale; sentences are
    interpolated, never concatenated from fragments. Why: a missing key shows the raw key to
    the user, and concatenation breaks grammar in other languages. Check: Grep (multiline)
    `[^=]>\s*[A-Za-zÀ-ÿ¡¿][^<>{}=;]*</` in `.tsx` → 0; Grep
    `(accessibilityLabel|title|placeholder|message)=["'][A-Za-zÀ-ÿ]` → 0; a test asserts the
    exact same key set in every locale file.
M9. **Design tokens only.** Colours, spacing, radii, typography and shadows come from the
    project's token source (theme file, token package or utility classes); components hold
    no hex/rgb/hsl literals, and light and dark both resolve through tokens. When `kit.json`
    `brand` is set, token values match that brand reference. Why: one-off values drift from
    the brand and break dark mode. Check: Grep `#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(` in
    `apps/mobile` `.tsx` files → hits only in token/theme files; token values
    compared with the brand reference when `brand` is set.
M10. **Phone sizes, safe areas and keyboard.** Every changed screen is checked in portrait
    on a small phone (360×640 dp Android / 375×667 pt iOS) and a large phone (about
    430×932): no clipped or overlapping text, no horizontal overflow, the primary action
    reachable. Every screen root goes through the project's ONE screen wrapper, which
    applies safe-area insets with explicit edges (no hand-written status-bar padding). On
    every form the keyboard never covers the focused field or the submit action, and each
    field sets its keyboard type, autofill hint (`textContentType` / `autoComplete`) and
    return key. Why: layout bugs show up on the smallest and the largest device, and a
    covered submit button blocks the flow. Check: the builder report holds both screenshots
    (or the exact run steps and result); Grep `SafeAreaView|StatusBar\.currentHeight` outside
    the screen wrapper → 0; email, phone and number fields set `keyboardType`.
M11. **Deep links are validated input.** Every incoming URL (custom scheme, universal/app
    link, push payload) goes through ONE pure parser that accepts only the app's own scheme
    and hosts and the known routes, ignores malformed URLs without crashing, removes the
    params it consumes, and accepts one-time tokens only on the route that redeems them.
    Navigation never targets a URL taken from the link as-is. Why: a link is
    attacker-controlled input; unvalidated, it is an open redirect into the app or a token
    left in navigation history. Check: parser specs cover a foreign host, a malformed URL,
    an unknown route and a token on the wrong route; Grep
    `Linking\.(getInitialURL|addEventListener)|useURL\(` → only in the deep-link hook.
M12. **Logs and error reporting through one adapter.** Shipped code has no `console.*`
    output and never logs a token, password, request/response body or personal data.
    Crashes and handled errors go through ONE reporting module (`captureException`,
    `trackEvent`, `identify`); features never import a vendor SDK. The module scrubs
    tokens, query strings and bodies, sends ids and enums only (no names, emails or free
    text), and is a no-op without its key. A root error boundary reports and shows a screen
    with a retry. With the `observability` addon, the `OB` rules also apply. Why: device logs
    and vendor dashboards have more readers than the database, and swapping a vendor must
    not touch features. Check: Grep `console\.(log|info|debug|warn)` in the source root → 0
    or behind a dev guard; Grep
    `from ['"](@sentry/|posthog|@bugsnag|@datadog|@react-native-firebase/crashlytics)` outside
    the reporting module → 0; a spec asserts the exact set of keys the scrubber keeps.
M13. **Version contract with the API.** The client sends its app version on every request
    and checks a server-provided minimum version at start; below it, a blocking screen
    links to the store. The gate fails OPEN (malformed version or failed request → not
    blocked). Contract changes work in both directions: a field the client starts sending
    is accepted by the deployed API first; a new required field is not imposed while
    published clients that lack it are above the minimum version; a response the old
    client cannot handle uses a code it already knows. Why: published apps live for weeks
    and are not redeployed with the API. Check: the gate's spec covers older, equal, newer
    and malformed versions and a failed request; Grep the HTTP client for the version
    header; any contract change carries a `Deploy order:` line in the commit body.
M14. **Release order and delivery channel.** The API deploys before (or with) the client
    that needs it, and the client ships only after that API is live in the environment the
    client calls (a preview build that calls the production API needs the change in
    production). What changed decides delivery: JS/TS and bundled assets only →
    over-the-air update for the matching runtime; any native change (native dependency
    added, removed or upgraded, permissions, app config, SDK upgrade, runtime version) →
    new native build and a version bump. An update is never applied by reloading the app
    mid-flow: download in the background, apply on the next cold start or after a user tap.
    Why: a client that ships before its endpoint fails in production, and an OTA that needs
    native code crashes at launch. Check: the builder report states `OTA` or `native build`
    with the changed files that decided it; Grep `reloadAsync\(` → only behind a user action.
M15. **Tests assert behaviour.** Tests run with `Vitest` (Vitest by default;
    Jest only where the project already uses it). Every new file with logic (hooks,
    helpers, services, mutations, stores, parsers) has a spec written FIRST and seen
    failing; every view with a conditional has a structure or render spec asserting what
    the user gets in each branch. Specs assert outcomes (returned values, calls at the HTTP
    boundary, rendered pieces and their props), never private implementation; native
    modules are mocked once in the setup file with the library's real contract and error
    shapes. The mutation check (`G3`) passes for every new or changed rule and the `FT`
    list is walked. Why: a test that does not fail when the rule breaks proves nothing.
    Check: Glob a sibling spec for each new file in `hooks|helpers|services|mutations|stores`;
    the mobile test command in `kit.json` exits 0; the builder report has the mutation table.
M16. **Numeric input on phones.** Amount and quantity fields parse locale input: the
    decimal keypad types a comma in many locales and users paste `1.234,56`, so both
    separators parse to one canonical value, the comma is never stripped, an empty field
    is "no value" (not `0`), and out-of-range input is marked invalid, never corrected.
    Why: stripping the comma turns `5,50` into `550`. Check: parser specs with `5,50`,
    `5.50`, `1.234,56`, `1,234.56`, `""` and `abc`; Grep `\[\^\\d\.\]|\[\^0-9\.\]` → 0.

M17. **Client state in Zustand, server state in TanStack Query.** The mobile twin of WA3
    and W4. Server data follows M2 (read through the shared query hooks, written through
    mutations) and is never copied into a store. Zustand holds only client state — UI
    (open sheet, selected tab, draft filters), session flags and app settings — one store
    per concern in the app's shared `stores/` folder (`use<Concern>Store`), each with its
    spec (M15). Components and hooks read a store through a selector
    (`useCartStore((s) => s.count)`; `useShallow` for several fields), never the whole
    store, which re-renders the caller on every change. A persisted slice goes through
    `persist` with an explicit storage adapter (`createJSONStorage(() => AsyncStorage)` or
    the project's adapter), `partialize` to the fields that must survive a restart, and a
    `version` (plus `migrate` when its shape changes). Secrets never go in a persisted
    store: they live in the secure store (M4, ME2). Why: a whole-store subscription
    re-renders every screen on every keystroke, and a server copy in a store is the stale
    data the query already refetched. Check: Grep `\buse[A-Z]\w*Store\(\s*\)` in the
    mobile scopes → 0; Grep `from ['"]zustand(/middleware)?['"]` outside `stores/` → 0
    (`useShallow` may be imported anywhere); Grep the stores for
    `useQuery|useMutation|fetch\(|axios|Service\.` → 0; every `partialize` keeps no token,
    password or secret field (M4).
<!-- /kit:block mobile-common -->

<!-- kit:block mobile-expo v1 -->
## Rules — Expo React Native (`ME`)

ME1. **Expo Router routes are thin.** Routes live under the router root (`app/` or
    `src/app/`, as set by the `expo-router` plugin `root` option). Each route file
    default-exports a screen that renders one feature root (`M1`) and nothing else;
    `_layout.tsx` files only declare navigators, providers and screen options; groups
    `(name)` organise without changing URLs; dynamic segments `[param]` are read with
    `useLocalSearchParams` inside the feature hook and validated before use. No components,
    hooks or helpers live under the router root (every file there becomes a route). The root
    layout exports an `ErrorBoundary` that reports through the reporting module (`M12`),
    `+not-found.tsx` exists, and `experiments.typedRoutes` is on. Why: route files cannot be
    unit-tested, and a stray file under the root ships as a screen. Check: Glob the router
    root → only route files, `_layout.tsx` and `+*.tsx`; Grep
    `use(State|Effect|Query|Mutation)\(|fetch\(` in route files other than layouts → 0; Grep
    `export function ErrorBoundary` in the root `_layout.tsx` → 1.
ME2. **`expo-secure-store` for secrets.** `M4` is implemented with `expo-secure-store`
    (`setItemAsync`, `getItemAsync`, `deleteItemAsync`) inside ONE session module, with key
    names as constants and small values (tokens, not blobs); `getItemAsync` is wrapped so a
    keystore error resolves to `null`. `@react-native-async-storage/async-storage` holds only
    non-secret preferences (theme, locale, drafts). `expo-secure-store` is listed in the app
    config `plugins`. Why: AsyncStorage is unencrypted. Check: Grep `SecureStore\.` → only in
    the session module; Grep -i `AsyncStorage\.setItem\([^)]*(token|secret|password|session)`
    → 0; a spec with `expo-secure-store` mocked asserts logout calls `deleteItemAsync` for
    every key.
ME3. **`EXPO_PUBLIC_*` only, read statically.** App code reads only
    `process.env.EXPO_PUBLIC_<NAME>` with literal dot access, inside the config module
    (`M5`): Metro inlines only that form, so `process.env[name]` or destructuring
    `process.env` is `undefined` on the device. Values come from the EAS environment
    (development / preview / production) or the build profile `env` in `eas.json`;
    `app.config.*` may read non-public variables at build time but never copies a secret
    into `extra`, which ships in the manifest. In development, without an explicit API URL,
    the base URL derives from Metro's host (`Constants.expoConfig?.hostUri`), because
    `localhost` on a phone is the phone. Why: the bundle is public, and dynamic env access
    fails silently on device. Check: Grep `process\.env\[|=\s*process\.env\s*;` → 0; every
    Grep `process\.env\.\w+` hit in app source is `EXPO_PUBLIC_*` or `NODE_ENV`; the `extra`
    object of the app config holds no secret.
ME4. **Randomness from `expo-crypto`.** Every identifier, nonce, idempotency key, payment or
    order reference generated on the device comes from `Crypto.randomUUID()` or
    `Crypto.getRandomValues()` of `expo-crypto`; never `Math.random()`, a timestamp, or a
    global `crypto` the JS engine may not provide. Tests mock `expo-crypto` with
    `node:crypto` (same contract) in the setup file. Why: `Math.random` and time values are
    predictable and collide. Check: Grep `Math\.random\(` in non-spec source → 0 outside
    visual effects (each such hit commented); Grep `Date\.now\(\)` in builders of ids, keys
    or references → 0.
ME5. **EAS profiles and runtime version.** `eas.json` declares `cli.appVersionSource` and
    the build profiles `development` (`developmentClient: true`, internal), `preview`
    (internal, `channel: "preview"`) and `production` (store, `channel: "production"`,
    `autoIncrement: true`); each profile's `channel` is the one its updates publish to. The
    app config declares `runtimeVersion` explicitly (`{ "policy": "appVersion" }`, which
    means bumping `version` on every native change, or `{ "policy": "fingerprint" }`) and
    `updates.url`. Why: a build and an update with different runtime versions never meet,
    and an update on the wrong channel never reaches its build. Check: read `eas.json`;
    `npx expo config --type public` shows `runtimeVersion` and `updates.url`;
    `npx expo-doctor` exits 0.
ME6. **OTA with `eas update`, native change with `eas build`.** `M14` with Expo. OTA when
    only JS/TS, assets or JS-only dependencies changed:
    `eas update --channel <channel> --environment <environment> --message "<why>"`; the
    `EXPO_PUBLIC_*` values of an update come from the EAS environment named by
    `--environment` (required from SDK 55), not from `eas.json`. Native build plus a
    `version` bump when the change touches a dependency with native code or a config plugin
    (the package has `ios/`, `android/`, `app.plugin.js` or `expo-module.config.json`), the
    app config (plugins, permissions, scheme, icons, splash, intent filters, entitlements),
    `ios/` or `android/`, the Expo SDK, or `runtimeVersion`:
    `eas build --profile <profile> --platform all`. Updates download in the background
    (`Updates.fetchUpdateAsync()`) and apply on the next launch; `Updates.reloadAsync()` runs
    only after a user tap. Bad update: `eas update:rollback`. Why: an OTA that needs native
    code crashes at launch on every installed build. Check: `git diff --name-only` of the
    change against the list above, and the `ls` of each new dependency's folders, recorded in
    the builder report.
ME7. **Dependencies through the Expo installer.** Expo and React Native packages are added
    with `npx expo install <package>` (SDK-compatible version), never a hand-picked version;
    `npx expo install --check` and `npx expo-doctor` exit 0 after any dependency change. Why:
    a version outside the SDK's set builds and then crashes in native code. Check: run both
    commands; their exit codes go in the builder report.
ME8. **Lists, images and the JS thread.** Long or paginated lists render with FlashList
    (`@shopify/flash-list`) or FlatList, never `ScrollView` plus `.map()` over server data;
    rows are `React.memo` components with a stable `renderItem`, `keyExtractor` returns a
    stable id (never the index), and pagination calls the query's `fetchNextPage` from
    `onEndReached`. Images use `expo-image` with a `cachePolicy`, a `recyclingKey` inside
    lists, and a source sized close to the rendered size (thumbnails, not originals). No
    heavy synchronous work in render or during gestures (transform in the query `select` or
    on the server); animations run on the UI thread (Reanimated, or `useNativeDriver: true`).
    Why: JS-thread work drops frames and unvirtualized lists exhaust memory. Check: Grep
    `import \{[^}]*\bImage\b[^}]*\} from 'react-native'` → 0; Grep
    `key=\{(index|i|idx)\}|=>\s*String\(index\)|index\.toString\(\)` → 0 on data rows
    (fixed placeholder arrays such as skeleton rows may use the index); Grep
    `useNativeDriver:\s*false` → each hit justified; `.map(` inside a scroll container over
    query data → 0 on review.
ME9. **One scroll primitive.** Every vertical scroll container uses the project's scroll
    wrapper: ONE file in the shared UI folder that wraps `ScrollView`, and the only file with
    a raw vertical `<ScrollView>`. The wrapper sets `contentInsetAdjustmentBehavior="never"`
    (insets come from the screen wrapper, `M10`), `alwaysBounceVertical` only when a
    `refreshControl` is passed, and `keyboardShouldPersistTaps="handled"`; props the caller
    passes win. Horizontal carousels and virtualized lists are out of its scope. Why: iOS adds
    its own safe-area insets on top of the screen wrapper's (the whole safe area inside a
    modal), so screens scroll past their content and feel stuck; the bug keeps coming back
    until the fix lives in one place. Check: Grep `(^|[^\w])<ScrollView\b` in
    `apps/mobile` `.tsx` files → each hit is the wrapper file, a comment, or a tag
    whose props include `horizontal` (read the whole tag); a spec covers the wrapper's
    defaults and caller overrides.
ME10. **Tests: Vitest in node, structure tests, optional Maestro.** Vitest runs with
    `environment: 'node'` (jsdom's fake `window` hides code that crashes on the device);
    native modules are mocked once in `vitest.setup.ts` with their real contract
    (`expo-constants`, `expo-secure-store`, `expo-crypto` through `node:crypto`, and any
    package published as Flow that Vite cannot parse); workspace packages are aliased to
    their source so tests run the code Metro bundles. Views get structure tests: mock the
    primitives as strings (`vi.mock('react-native', () => ({ View: 'View', Text: 'Text',
    Pressable: 'Pressable' }))`) and the hooks the view calls, call the dumb component as a
    function, walk the returned tree, and assert which pieces render with which props in each
    branch. A project already on Jest keeps `jest-expo` with `@testing-library/react-native`
    and asserts by role and text. End-to-end golden flows are optional with Maestro
    (`maestro test .maestro/`) on a development build. Why: logic tests in node are fast and
    deterministic, and a renderer-free structure test still pins what the view shows. Check:
    the Vitest config sets `environment: 'node'`; the mobile test command in `kit.json`
    exits 0; each changed view with a conditional has a structure or render spec.
ME11. **iOS and Android differences are explicit.** Platform-specific behaviour goes through
    `Platform.select` / `Platform.OS` inside hooks or helpers (each branch unit-tested with
    `Platform.OS` mocked) or through `.ios.tsx` / `.android.tsx` files with the same props.
    Known differences are handled on purpose: Android hardware back (`BackHandler` in modals
    and multi-step flows), Android edge-to-edge insets, iOS sheet presentations
    (`formSheet` / `modal`), keyboard avoidance (`padding` on iOS, `height` on Android) and
    permission prompts. Every changed screen runs on both platforms before "done", or the
    builder report names the platform not verified. Why: the platform you did not run is the
    one that breaks. Check: Grep `Platform\.OS\s*===|Platform\.select\(` in changed files →
    each branch has a spec; the builder report lists the platforms verified.
ME12. **App identity declared once.** App name, slug, scheme, bundle identifier/package,
    owner, EAS project id, update URL, web domain and store URLs are declared once (app
    config or the config module) and read from there (`Constants.expoConfig`, config module
    exports); the scheme used in code equals `expo.scheme`; feature code never repeats these
    literals. Why: a duplicated scheme or domain drifts on the next rename and links open the
    browser instead of the app. Check: Grep the scheme and domain values from the app config
    in the source root → only the config module.
ME13. **Permissions.** The app declares only the permissions it uses: iOS usage strings in
    the plugin options or `ios.infoPlist`, localized through `expo.locales`; Android
    permissions added by libraries but unused go in `android.blockedPermissions`. A
    permission is requested at the moment the feature needs it, after an in-app explanation,
    and `denied` / `blocked` leads somewhere (`Linking.openSettings()`), never to a dead end.
    Why: stores reject vague or unused permissions, and a denied prompt must not strand the
    user. Check: compare the app config permissions with Grep
    `request\w*PermissionsAsync|requestPermission` call sites; each call site's hook has a
    spec for the denied branch.
ME14. **Expo runs in its own terminal.** The dev server (`npx expo start` or the project's
    mobile dev script) runs in a separate interactive terminal opened by a person: never
    inside a task-runner pipeline (Metro needs a TTY for the QR code and key presses and dies
    when its output is piped) and never in an agent's foreground shell (it never exits). In a
    monorepo, the workspace `dev` script excludes the mobile app and a separate script starts
    it. Agents check Metro with `curl -s http://localhost:8081/status`
    (`packager-status:running`) and ask the person to start it when needed. Why: a piped or
    captured Metro hangs the session and serves nothing. Check: in a monorepo, the root
    `package.json` `dev` script excludes the mobile package and a dedicated mobile dev script
    exists.

ME15. **Styling: uniwind + Tailwind v4, tokens only.** Styles are Tailwind v4 classes through
    `uniwind` (`className` on React Native components); NativeWind only when the project
    already uses it (record it in the decisions/ADRs). One root `global.css`:
    `@import 'tailwindcss'; @import 'uniwind';`, then the UI library's theme if any, then
    the brand tokens LAST in `@theme` (in Tailwind v4 the last definition wins, so a library
    theme imported after the brand silently overrides `--color-surface` and friends). Brand
    tokens come from `docs/brand.md` or the monorepo's shared design-tokens package, never
    retyped. Declare `@source` for every folder that holds classes (`app/` routes AND `src/`
    features): Tailwind only generates classes it finds, and a folder it misses renders
    screens at zero height — a blank app with no error. `metro.config.js` wraps every other
    Metro wrapper with `withUniwindConfig(...)` (outermost) and sets `dtsFile` so class
    names are typed. Components use semantic token classes (`bg-surface`, `text-text`,
    `bg-brand-500`, `rounded-card`), never hex/rgb literals or raw `StyleSheet` colors.
    Why: one token file drives web and mobile alike, and the two silent failures above
    (theme order, missing `@source`) each cost a debugging day. Check: `package.json` has
    `uniwind` and `tailwindcss@4` (and no `nativewind` unless recorded); `global.css`
    imports in that order with `@source` lines covering `app/` and `src/`; Grep
    `withUniwindConfig` in `metro.config.*` is the outermost call; Grep
    `#[0-9a-fA-F]{3,8}\b|rgba?\(` in every mobile scope's `*.tsx` → only token files.
<!-- /kit:block mobile-expo -->

<!-- kit:block addon-money v1 -->
## Rules — Money, idempotency and locks (`MO`)

Applies to anything that charges, pays, credits, refunds, reserves, prices or meters.
Every money fix brings the attacker test and the third-party-down test (G2). Base rules
this block builds on instead of repeating: decimal types (A6, AN9), decimal input (A3, W11,
M16), transactions and atomic locks (AN5, AN6 — the same mechanics apply to any backend),
long work off the request (A10) and the backward-compatible contract with its deploy order
(A18, M13).

MO1. **Cents and rounding.** On top of A6: amounts are split into cents with the
     largest-remainder method, never `Math.floor(x * 100)`; reserve, check and release call
     the SAME function with the same rounding; a tolerance is one-sided; any comparison with
     `NaN` stops the operation. Why: 4.35 × 100 is 434.999…, an invoice lost a cent, and
     two rounding paths let a reserved cent look free. Check: Grep
     `Math\.(floor|trunc|round)\([^)]*\*\s*100`, `Math\.abs\(` inside an amount tolerance
     and `parseFloat\(` in money code → findings.

MO2. **Client-generated idempotency key.** Every action that moves money or prices carries
     an idempotency key generated by the CLIENT that survives closing the dialog, reloading
     and unmounting (persisted per scope, e.g. session storage for 24 h), bound to the
     content (same key with a different payload → 409), recorded in the SAME transaction as
     the effect, and REQUIRED in the DTO, the client type and the service signature. A
     disabled button is not idempotency. Why: a proxy timeout plus a retry applied a price
     increase twice. Check: Grep money DTOs and service signatures for an idempotency field
     declared optional (`?:`, `.optional()`, `@IsOptional`) → finding; a test replays the
     same key (one effect) and the same key with another payload (409).

MO3. **The same money never moves twice.** Money writes use the AN5/AN6 mechanics (lock
     as the first statement, keys in one global order, decide inside the transaction, no
     catch-and-continue, side effects after commit) and, in addition, either a serializable
     transaction wrapped in ONE retry helper (serialization failure → retry; on giving up,
     409 with a code such as `operation.in_progress`) or a conditional update whose row
     count is checked. Every READ COMMITTED writer of the same balance (refunds, reversals,
     debt payments) waits for the same locks. Why: a swap read the free balance from before
     a payment's lock and the same money left twice. Check: Grep `isolationLevel` → each
     call goes through the retry helper; a concurrency test fires N parallel requests and
     exactly one effect happens.

MO4. **Payment rails.** Settling, confirming, capturing or crediting re-reads the payment
     at the provider by OUR reference; an id or amount coming from the client never pays,
     delivers or credits anything. "Not confirmed" (pending, foreign, closed order) is a 4xx
     with a code, never a 2xx. A webhook processes only what its signature covers and
     re-reads the state at the provider. Uncertain is not "no": a 5xx or timeout from the
     provider never refunds, cancels, deletes or marks failed blindly — a sweep asks again
     and settles late approvals through the SAME path, and a critical incident is opened
     ONCE per fact (deduplicated by the purchase, grouped by fingerprint). Why: a made-up
     transaction id paid a real order. Check: Grep settle/confirm/capture handlers for DTO
     fields used as the provider id or amount; a test sends a foreign reference → 4xx; a
     test with the provider returning 5xx → no state change and one incident.

MO5. **Record at request time.** Whatever decides money and can change between request and
     payment (payout account, country cost, price, invoice series, exchange rate) is stored
     with the request and read from there, never recomputed with today's settings. Before an
     irreversible external effect everything is validated and the balance is RESERVED under
     a lock; it is released only when it is known the effect did not happen; the id given
     to the third party is one per attempt. Why: a payout approved for one account was paid
     to another after a settings change. Check: Grep the settle/release path for a
     recomputation of a stored value.

MO6. **Money tests.** Each money rule has: the boundary pair (last passing, first failing —
     FT1), rounding fixtures that pin the direction (FT13), the attacker request, the
     provider-down case, and a concurrency test for every lock (N parallel → one effect).
     Why: money bugs pass happy-path suites. Check: `auditor-tests` lists these five per
     touched money rule.
<!-- /kit:block addon-money -->

<!-- kit:block addon-tenancy v1 -->
## Rules — Multi-tenancy (`T`)

The tenant is the organization (key `organizationId` / `organization_id` unless a `P` rule
names another). A leak between tenants is always a BLOCKER. Base rules this block builds on
instead of repeating: the 404 envelope (A7), default-deny route markers (A14, AN10), private
storage keys (A13, AN12) and `undefined` dropped from a `where` (AN7).

T1. **Every business row belongs to a tenant.** Every business table has the tenant key
    (NOT NULL, indexed) and every query on it filters by the tenant in the service or
    repository — the first barrier. Where the database supports it, row-level security keyed
    by a per-transaction setting is the second barrier, added in the same migration that
    creates the table. Why: one missing `where` exposes every customer. Check: Grep touched
    data-access files for `find(Unique|First|Many)\(\s*\{\s*where:\s*\{\s*id` and
    `(update|delete)(Many)?\(\s*\{\s*where:\s*\{\s*id` without the tenant key on the same
    call; every new table's migration contains its policy (`CREATE POLICY` / `ENABLE ROW
    LEVEL SECURITY`) or a `P` rule says why not.

T2. **The other-tenant test, per module.** Another tenant's resource answers exactly like a
    missing one (404 with the resource's code, A7). Every module ships an integration test
    that creates two tenants with different data (FT5) and tries to read, update, delete,
    list, search and export the other tenant's resource → 404 (FT11); the tenant filter is
    a mutation target of G3. Why: a 403 confirms the id exists, and only this test proves
    the filter. Check: Grep each touched module's integration spec for the other-tenant
    case.

T3. **The tenant comes from the session.** The active tenant is resolved per request from
    the authenticated session (a header that selects among the user's tenants is checked
    against membership); a tenant id in a body or query is never trusted. Services receive a
    request context (`ctx.orgId`, `ctx.userId`, `ctx.role`), not loose ids. Why: a forged
    id would otherwise pick the tenant. Check: Grep DTOs for `organizationId|orgId|tenantId`
    as an input field → finding unless a `P` rule allows it (e.g. a platform backoffice).

T4. **No role escalation.** The permission matrix lives in ONE shared module (route markers
    themselves: A14, AN10). Nobody can grant, invite or promote above their own role, and a
    user can never change their own role or permissions; changing the last owner is
    refused. Why: an admin who makes themselves owner owns the tenant. Check: one test per
    escalation path (invite with a higher role, self-promotion, demoting the last owner)
    expects the error code.

T5. **Scope contracts.** Tenant-scoped data access takes a typed scope (a branded type that
    only the request context or a named system scope — `fromSystem(orgId, 'reason')` —
    can create); casting into it is a lint error. Access that bypasses tenant isolation
    (`asSystem('reason', fn)`) is allowed only from files listed in one allowlist file, each
    with a snake_case reason. Background jobs carry the tenant explicitly in their payload.
    Why: the type system makes "forgot the tenant" a compile error. Check: Grep
    `as\s+(OrgScope|TenantScope)` → zero; Grep the bypass call and compare its files with
    the allowlist; Grep job payload types for the tenant key.

T6. **Cross-tenant references are rejected.** A foreign key from a tenant row to another
    tenant table must point to a row of the SAME tenant — enforced by the database
    (composite FK or a same-tenant trigger) or, at least, by the service — and a violation
    maps to 404. Why: linking another tenant's id leaks it through joins. Check: every new
    FK column in a migration is in the guard (trigger/composite key); an integration test
    links a foreign tenant's id → 404.

T7. **Visibility in one helper.** Role visibility (own records, team, everything) is
    resolved by ONE helper (`visibilityWhere(ctx)`) used by every door: list, detail,
    search, counts, export, notifications. Why: the door that rebuilt the rule is the one
    that leaks (G12). Check: Grep touched queries for role checks written inline
    (`role ===`, `ctx.role` inside a `where`) instead of the helper.

T8. **Tenant ledgers are append-only.** Audit log, usage events and similar ledgers are
    insert-only (no update/delete in code; DB grants or triggers enforce it where possible).
    Creating, changing, deleting, exporting and viewing sensitive data is recorded in the
    same transaction, with minimal before/after and no secrets. Why: an editable audit log
    is not evidence. Check: Grep `(update|delete)(Many)?\(` on ledger models → zero; the
    sensitive action's test asserts its ledger row.

T9. **The tenant is the owner everywhere.** In storage keys (A13: `org/<orgId>/<kind>/<uuid>`),
    signed URLs, search indexes, caches, queues and exports, the owner segment is the
    tenant, and a key or entry of another tenant is 404. Tenant and visibility filters are
    built from required, validated values (AN7: an `undefined` filter returns every row).
    Why: every secondary store is another door. Check: a test signs, searches and exports
    with another tenant's key → 404; Grep cache/search keys for the tenant segment.
<!-- /kit:block addon-tenancy -->

