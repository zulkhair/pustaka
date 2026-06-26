# Pustaka — Design Spec (v1)

- **Date:** 2026-06-26
- **Status:** Draft for review
- **Type:** Personal / OSS, self-hosted, multi-user (open self-signup)

---

## 1. Summary

**Pustaka** ("library / repository of texts") digitalizes physical documents and books. A
Flutter mobile app captures pages; a Go backend stores them and runs a **two-stage local-AI
pipeline** — per-page OCR (**GLM-OCR**) then a **template-driven transform** (**qwen2.5:14b**) —
turning scans into a chosen output format. A Kindle/Google-Books-style reader browses the
library. All inference runs **locally via Ollama on the GPU box `msi`** over Tailscale; nothing
leaves the host. It is **multi-user** with **email-verified** open self-signup (Resend, behind a swappable mail port) — each account's library is fully private.

**Long-term vision:** a general "scan → transform" engine where *templates* produce any of:
structured data, a clean reformatted document, a filled artifact, or just searchable text.
**v1 is a thin vertical slice** that proves every layer end-to-end with **two seed templates**,
on an architecture ready to grow into the full engine.

This reuses the proven **invoice-extractor** pipeline patterns (GLM-OCR `→` map two-model path,
lenient JSON parse, schema validation, GPU-free testing).

---

## 2. Goals / Non-goals

**Goals (v1)**
- Capture a multi-page document on mobile (photo per page, on-device compression).
- Two capture modes per document: **Photo** (keep compressed image + OCR) / **Text** (OCR, discard image).
- Per-page OCR via GLM-OCR, incremental and reviewable as you capture.
- A **template engine** that transforms OCR text into a chosen output; ship **2 seed templates**.
- A **library** + a **reader** view (swipe pages: image when present, else text).
- Store templates, OCR results, and outputs in Postgres; page images on a filesystem volume.
- Self-host on the dev box behind Caddy, like the other apps.
- **Multi-user:** open self-signup (register/login), per-user data isolation, and an admin role.
- **Email-verified registration:** confirm the address before activation, via **Resend** behind a swappable **Mailer port**.

**Non-goals (v1)** — deferred, not designed now
- Orgs / teams, **sharing** documents or templates between users, billing, or per-user quotas (flat user list; each user's data is fully private; built-in templates are shared).
- Password reset / magic-link login / 2FA (later — the same Mailer port enables them).
- Offline-first capture + sync (v1 is online-only).
- User-defined templates (v1 ships built-in templates only).
- Polished ePub/PDF export, advanced page edge-detection/deskew beyond a basic crop.
- Handwriting-specific tuning (GLM-OCR handles what it handles).

---

## 3. Users & scope

Multi-user, self-hosted. **Open self-signup** (register + login; JWT + bcrypt, pasar's auth
extended). Each user's documents/outputs are **fully private** (owner-scoped); built-in templates
are shared (global). One **admin** role (seeded) manages users + built-in templates; everyone else
is a regular user. Scale target: a handful of users, a personal library each. Open signup means
basic rate-limiting/validation on registration (see §11), especially if the instance is exposed.
Registration requires **email verification** (a one-time code emailed via Resend) before the
account is activated and can obtain a session.

---

## 4. Architecture — ports & adapters (hexagonal-lite)

The **core** (entities + use-cases) depends only on **port interfaces**. Postgres, the
filesystem blob store, and Ollama are **adapters** behind those ports. Payoff: the whole app is
testable **without a GPU or Postgres** (mock the ports) — the discipline that lets
invoice-extractor test its pipeline GPU-free. The **AI pipeline is one isolated module** with a
tiny surface, so models/prompts evolve without rippling outward.

**Ports**
```
AIClient:   Transcribe(ctx, imageBytes) → markdown            // GLM-OCR via /api/generate
            Transform(ctx, ocrText, template) → output         // qwen2.5:14b via /api/chat
BlobStore:  Put(docID, page, bytes) / Get / Delete / Thumbnail
Store:      sqlc Queries + ExecTx (transactions)
Mailer:     SendVerificationCode(ctx, email, code)  (+ generic Send)   // Resend adapter; swappable
```

**Flow**
```
Mobile ──capture/compress──> HTTP API ──> app/document (store page + blob)
                                   │
                                   ├─ app/ocr        → AIClient.Transcribe (GLM-OCR)  → ocr_result
                                   └─ app/transform  → AIClient.Transform (qwen2.5)   → output
Adapters: store(Postgres)  blob(filesystem)  ai(Ollama@msi via Tailscale)
```

**Ownership** is enforced in the app/store layer: every `document`/`output` query is scoped by the
authenticated user's `user_id`; handlers never trust a client-supplied owner. Admins may
additionally manage built-in (global) templates and users.

---

## 5. Data model (Postgres)

| Table | Key columns | Notes |
|---|---|---|
| `web_user` | id, username, **email** (unique), password_hash, **role** (`admin`\|`user`), **email_verified** (bool), created_at | new users get `user` + `email_verified=false`; a seeded admin exists (pre-verified) |
| `email_verification` | id, user_id, **code_hash**, expires_at, attempts, consumed_at, created_at | single-use 6-digit code; **hashed**; 15-min expiry; max 5 attempts; throttled resend |
| `session` | id, user_id, **refresh_token_hash**, expires_at, created_at, revoked_at | refresh tokens stored **hashed**; enables logout / revocation / rotation |
| `document` | id, **user_id** (owner), title, **mode** (`photo`\|`text`), page_count, status, created_at | a captured doc; owner-scoped |
| `page` | id, document_id, page_number, **image_path** (nullable), **thumb_path** (nullable), width, height, status | image_path null in text mode (or after discard) |
| `ocr_result` | id, page_id, model, text (Markdown), status, created_at | per page; re-runnable, latest wins |
| `template` | id, **owner_user_id** (nullable), name, doc_type_hint, **scope** (`page`\|`document`), prompt, **output_format** (`markdown`\|`json`\|`csv`\|`text`), json_schema (nullable), is_builtin | `owner_user_id` null = built-in/global; per-user templates later |
| `output` | id, **user_id**, document_id, template_id, content (text/JSON), file_path (nullable), model, status, created_at | one row per transform **run**; for `page`-scope templates `content` is an array (one entry per page, keyed by `page_number`), for `document`-scope a single artifact. A doc can have many outputs (different templates / re-runs). |

`status` everywhere ∈ `pending|processing|done|failed`. **Ownership lives on `document.user_id` and `output.user_id`; `page`/`ocr_result` inherit it via `document_id`** — every read filters by the authenticated user. Money/precision not relevant here.

---

## 6. Pipeline

**Two stages, deliberately separated** (OCR and transform have different context needs):

1. **Capture (mobile)** — shoot a page; compress on device (resize longest edge ~2048px, JPEG
   q~80 → ~200–500 KB); upload to `POST /documents/:id/pages`.
2. **Store** — backend saves the image (Photo mode) + a ~400px thumbnail; creates the `page`.
   Text mode: image is used for OCR then **discarded** (not persisted).
3. **OCR (per page, incremental, reviewable)** — GLM-OCR transcribes the page image → Markdown
   → `ocr_result`. Runs right after each page so the user can review/re-shoot immediately.
4. **Transform (scoped)** — user picks a template:
   - `scope=page` → run per page over that page's OCR text.
   - `scope=document` → run once over all pages' OCR text (page-marked), enabling reflow,
     cross-page structure, and running-header dedup.
   Output parsed/validated per `output_format` (JSON validated against `json_schema`) → `output`.
   Each transform **run** writes one `output` row: page-scope → `content` is an array of per-page
   results (keyed by `page_number`); document-scope → a single artifact.

Rationale for per-page OCR + scoped transform is recorded in §17.

---

## 7. Models (Ollama on `msi`, over Tailscale)

| Stage | Model | Endpoint | Why |
|---|---|---|---|
| OCR | `glm-ocr:latest` (1.1B specialist) | `/api/generate` (capped `num_predict`) | purpose-built transcription; proven in invoice-extractor |
| Transform | `qwen2.5:14b-instruct` | `/api/chat` (`format=json` for structured) | strong general text→structured/reformat mapper |

Configurable via env (`OLLAMA_HOST`, `OCR_MODEL`, `TRANSFORM_MODEL`). `OLLAMA_HOST` points at
`http://100.65.255.51:11434` (msi tailnet) in the live deployment.

---

## 8. API surface (Go/Fiber, `/api/*`, JWT)

- `POST /auth/register` (open signup → unverified user, emails a code) · `POST /auth/verify-email` `{email, code}` (activate → tokens) · `POST /auth/resend-verification` `{email}` (throttled) · `POST /auth/login` (rejected until verified) · `POST /auth/refresh` (rotate) · `POST /auth/logout` (revoke) · `GET /auth/me`
- `POST /documents` `{title, mode}` · `GET /documents` (library + thumb URLs) · `GET /documents/:id`
- `POST /documents/:id/pages` (multipart `file`) → store (+ blob if photo) → OCR → return text
- `GET /documents/:id/pages/:n/image` · `.../thumb`
- `POST /documents/:id/pages/:n/ocr` (re-run OCR) 
- `GET /templates`
- `POST /documents/:id/transform` `{template_id}` → run → `output`
- `GET /outputs/:id` (+ `?export=` for file formats, later)
- `GET /version` (+ `/download`) for mobile OTA (pasar/HAKA pattern)
- `GET /health` (reports Ollama up/down — invoice-extractor pattern)

Response envelope `{status, message, data}` (pasar convention). All `/documents` and `/outputs`
(and future user `/templates`) endpoints are **owner-scoped** to the JWT principal; admin-only
endpoints manage users and built-in templates.

---

## 9. Mobile app (Flutter, feature-first)

Screens: **Register / Verify-email / Login** · **Library** (thumbnail grid, mode badge, status) · **Capture** (new doc →
title + mode → camera, compress, upload, see per-page OCR, add next / finish) · **Reader**
(Kindle-like: swipe pages, pinch-zoom, **image⇄text** toggle, view outputs) · **Transform**
(pick template → run → view rendered output → export/share) · **Templates** (browse).

State: Riverpod; nav: go_router; HTTP: dio with an interceptor that **refreshes on 401** then retries (logout if refresh fails). Tokens (access + refresh) kept in **flutter_secure_storage**, not plain SharedPreferences.

---

## 10. Storage & compression

- **On capture (mobile):** resize to ~2048px longest edge, JPEG q~80 → ~10–20× smaller, faster upload. No OCR penalty (GLM-OCR reads fine at this resolution; invoice-extractor clamps to 1600).
- **Server:** store the compressed image (Photo mode) + a ~400px thumbnail under `BLOB_DIR/<user>/<doc>/<page>.jpg`. Text mode: never persist the image.
- **Outputs:** text/JSON in Postgres; binary exports (CSV/ePub/PDF) generated on demand later.

---

## 11. Security, error handling & status

**Security (first-class — "very good" by design):**
- **Passwords:** bcrypt (cost ~12, `pkg/hash`); argon2id is a drop-in upgrade behind the same helper.
- **Sessions:** short-lived **access JWT** (~15 min) + **refresh token** (rotating, stored **hashed** in `session`, revocable on logout); JWT secret in env, never committed.
- **Email verification:** 6-digit **CSPRNG** code, stored **hashed**, **single-use**, **15-min** expiry, **max 5 attempts**, **throttled resend**; **unverified accounts cannot obtain a session**.
- **Rate-limiting** on `register`/`login`/`verify-email`/`resend`/`refresh` (per-IP + per-account); login backoff/lockout.
- **Enumeration-resistant:** generic responses on register/resend/verify; **constant-time** comparison of codes/secrets.
- **Transport & secrets:** HTTPS only (Caddy); `RESEND_API_KEY`, JWT secret, DB creds in gitignored env; Resend uses a **verified sending domain** (SPF/DKIM/DMARC) so mail isn't spoofable.
- **Authorization:** owner-scoping (§4/§5) enforced server-side on every request; admin checks on admin-only routes.

**Operational:**

- Every `page`/`ocr_result`/`output` carries a status; incremental per-page means partial
  progress survives a failure; per-page **retry** in the UI.
- **`msi` offline / Tailscale down:** `/health` reports Ollama down; OCR/transform can be re-run
  later; captured pages/images are never lost.
- Upload failures retry; oversized/invalid files rejected with clear errors.
- **Open signup** gets input validation + basic rate-limiting on `/auth/register` and `/auth/login`; if the instance is exposed, front it with Caddy and consider abuse protection.

---

## 12. Testing

- **Backend:** Go integration tests with **testcontainers-Postgres** (pasar pattern); unit-test
  the template engine's deterministic parts (prompt build, output parse/validate) with a
  **mocked `AIClient`** — no GPU in CI, exactly like invoice-extractor. Plus an **owner-isolation** test (user A cannot read or modify user B's documents/outputs) — security-critical. Auth flow tested with a **mocked `Mailer` port** (no real email): verify-code happy path, expiry, attempt cap, unverified-login rejection, refresh rotation + revocation, and rate-limit behavior.
- **Mobile:** smoke/widget tests for capture→OCR and reader (later; minimal in v1).
- **CI:** GitHub Actions — `go vet`/`go test`; Flutter analyze/test.

---

## 13. Directory layout

**Backend (`backend/`)**
```
cmd/server/main.go                 # composition root: config → adapters → services → http
internal/
  config/                          # env (OLLAMA_HOST, model tags, BLOB_DIR, JWT secret, DB, RESEND_API_KEY, MAIL_FROM)
  domain/                          # entities + PORT interfaces only: AIClient, BlobStore, Store, Mailer
  app/{document,ocr,transform,template}/   # use-cases (orchestration)
  adapter/
    httpapi/{handler,middleware,router.go} # driving adapter (Fiber)
    store/{sqlc,queries,migrations}        # pgx + sqlc (generated; do not edit sqlc/)
    ai/{client.go,prompts,parse}           # Ollama AIClient (GLM-OCR + qwen2.5); ported prompts/parse
    blob/                                  # filesystem images + thumbnails
    mail/                                  # Mailer port impl: Resend adapter (swappable: SMTP/SES later)
  pkg/{jwt,hash}
db/{migrations,queries,seed.sql}   # seed = admin user (pre-verified) + the 2 built-in templates
```

**Mobile (`mobile/lib/`)**
```
core/{api,auth,router,theme,di,capture,error}
features/{library,capture,reader,transform,templates}/{data,application,presentation}
shared/widgets/
```

**Repo (monorepo)**
```
pustaka/ backend/ mobile/ docs/ scripts/(setup, pull_models) docker-compose.yml .prototools README LICENSE
```
(`CLAUDE.md` will be gitignored, mirrored into `~/_knowledge`, like invoice-extractor.)

---

## 14. Seed templates (v1)

1. **Clean Markdown document** — `scope=document`, `output_format=markdown`. Assemble pages into
   readable Markdown; fix OCR artifacts; preserve headings; drop repeated running
   headers/footers/page numbers. Proves the document-reflow family.
2. **Structured fields → JSON** — `scope=page`, `output_format=json`, with a small `json_schema`.
   Extract the key fields found on the page into a validated object. Proves the data-extraction
   family (the invoice-extractor lineage).

---

## 15. Deployment

Self-host on the dev box like pasar: Go API in docker compose (bound to `127.0.0.1`), Postgres,
a `BLOB_DIR` volume, behind a Caddy subdomain (`pustaka.dev.etracrown.web.id`, wildcard DNS +
Let's Encrypt). `OLLAMA_HOST` → `msi` over Tailscale. Mobile APK via the `/version` OTA pattern.
**(The app now has its own accounts + open signup, so it relies on app-level auth + signup
rate-limiting — *not* Caddy `basic_auth`, which would block registration.)**

---

## 16. Future (post-v1)

User-defined templates; more template families (filled forms, CSV/XLSX, ePub/PDF export); the
engine/app split (extract a stateless transform service); offline-first capture + sync;
batch/whole-document OCR options; better crop/deskew; orgs/teams, sharing documents/templates between users, per-user quotas; **password reset, magic-link login, 2FA, and extra Mailer adapters (SMTP/SES)** — all enabled by the Mailer port.

---

## 17. Decisions log (rationale)

- **Two-stage pipeline (per-page OCR + scoped transform).** OCR is inherently per-image and
  benefits from incremental review/retry; transforms sometimes need whole-document context.
  Splitting them, with a `scope` on each template, gets both — per the trade-off analysis.
- **Keep compressed image by default, text-only as a per-doc mode.** Compression already makes
  images cheap (~200–500 KB/page), so retaining them preserves the source of truth, the
  image-based reader, and re-OCR/re-transform; text mode exists for minimal-storage/privacy.
- **GLM-OCR + qwen2.5:14b two-model path.** Matches the invoice-extractor OCR→map design that's
  already validated and running on `msi`.
- **Ports & adapters.** Three genuinely distinct, slow/external infra concerns (DB, blob, AI)
  justify the seams; everything else stays pasar-flat. Not over-built.
- **Multi-user, open self-signup.** Ownership designed in from the start (every doc/output
  owner-scoped; templates global vs per-user) rather than retrofitted. Kept to a flat user list +
  one admin role — no orgs/teams/sharing/billing in v1 — so it stays a single implementation plan.
- **Email-verified signup behind a `Mailer` port.** Resend is one adapter; SMTP/SES (and later
  password-reset / magic-link) drop in without touching call sites. Security hardening (hashed
  single-use codes, rate-limits, refresh-token rotation, enumeration-resistance) is specified up
  front given the "very good security" requirement.

---

## 18. Open questions

- Structured-template UX: does the user supply field names per run, or pick a preset schema? (v1
  ships one preset; revisit when user-defined templates land.)
- Output export formats priority for v2 (CSV vs ePub vs PDF).
- Email verification UX: **6-digit code** (chosen — best mobile UX) vs magic-link (needs deep-linking). Flip if you prefer links.
- Confirm the **access + refresh** token model (chosen for security) vs a single longer-lived JWT (simpler).
