# Pustaka — Design Spec (v1)

- **Date:** 2026-06-26
- **Status:** Draft for review
- **Type:** Personal / OSS, self-hosted, single-user

---

## 1. Summary

**Pustaka** ("library / repository of texts") digitalizes physical documents and books. A
Flutter mobile app captures pages; a Go backend stores them and runs a **two-stage local-AI
pipeline** — per-page OCR (**GLM-OCR**) then a **template-driven transform** (**qwen2.5:14b**) —
turning scans into a chosen output format. A Kindle/Google-Books-style reader browses the
library. All inference runs **locally via Ollama on the GPU box `msi`** over Tailscale; nothing
leaves the user's machines.

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

**Non-goals (v1)** — deferred, not designed now
- Multi-user / SaaS / accounts beyond a single self-host login.
- Offline-first capture + sync (v1 is online-only).
- User-defined templates (v1 ships built-in templates only).
- Polished ePub/PDF export, advanced page edge-detection/deskew beyond a basic crop.
- Handwriting-specific tuning (GLM-OCR handles what it handles).

---

## 3. Users & scope

Single user, personal/self-hosted. Minimal JWT auth (one seeded user), reusing pasar's auth.
No multi-tenant data isolation. Scale target: personal libraries (hundreds of documents).

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
```

**Flow**
```
Mobile ──capture/compress──> HTTP API ──> app/document (store page + blob)
                                   │
                                   ├─ app/ocr        → AIClient.Transcribe (GLM-OCR)  → ocr_result
                                   └─ app/transform  → AIClient.Transform (qwen2.5)   → output
Adapters: store(Postgres)  blob(filesystem)  ai(Ollama@msi via Tailscale)
```

---

## 5. Data model (Postgres)

| Table | Key columns | Notes |
|---|---|---|
| `web_user` | id, username, password_hash, role | single seeded user (pasar pattern) |
| `document` | id, title, **mode** (`photo`\|`text`), page_count, status, created_at | a captured doc |
| `page` | id, document_id, page_number, **image_path** (nullable), **thumb_path** (nullable), width, height, status | image_path null in text mode (or after discard) |
| `ocr_result` | id, page_id, model, text (Markdown), status, created_at | per page; re-runnable, latest wins |
| `template` | id, name, doc_type_hint, **scope** (`page`\|`document`), prompt, **output_format** (`markdown`\|`json`\|`csv`\|`text`), json_schema (nullable), is_builtin | seeded built-ins in v1 |
| `output` | id, document_id, template_id, content (text/JSON), file_path (nullable), model, status, created_at | one row per transform **run**; for `page`-scope templates `content` is an array (one entry per page, keyed by `page_number`), for `document`-scope a single artifact. A doc can have many outputs (different templates / re-runs). |

`status` everywhere ∈ `pending|processing|done|failed`. Money/precision not relevant here.

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

- `POST /auth/login`
- `POST /documents` `{title, mode}` · `GET /documents` (library + thumb URLs) · `GET /documents/:id`
- `POST /documents/:id/pages` (multipart `file`) → store (+ blob if photo) → OCR → return text
- `GET /documents/:id/pages/:n/image` · `.../thumb`
- `POST /documents/:id/pages/:n/ocr` (re-run OCR) 
- `GET /templates`
- `POST /documents/:id/transform` `{template_id}` → run → `output`
- `GET /outputs/:id` (+ `?export=` for file formats, later)
- `GET /version` (+ `/download`) for mobile OTA (pasar/HAKA pattern)
- `GET /health` (reports Ollama up/down — invoice-extractor pattern)

Response envelope `{status, message, data}` (pasar convention).

---

## 9. Mobile app (Flutter, feature-first)

Screens: **Login** · **Library** (thumbnail grid, mode badge, status) · **Capture** (new doc →
title + mode → camera, compress, upload, see per-page OCR, add next / finish) · **Reader**
(Kindle-like: swipe pages, pinch-zoom, **image⇄text** toggle, view outputs) · **Transform**
(pick template → run → view rendered output → export/share) · **Templates** (browse).

State: Riverpod; nav: go_router; HTTP: dio with JWT interceptor + 401 logout (HAKA/pasar pattern).

---

## 10. Storage & compression

- **On capture (mobile):** resize to ~2048px longest edge, JPEG q~80 → ~10–20× smaller, faster upload. No OCR penalty (GLM-OCR reads fine at this resolution; invoice-extractor clamps to 1600).
- **Server:** store the compressed image (Photo mode) + a ~400px thumbnail under `BLOB_DIR/<doc>/<page>.jpg`. Text mode: never persist the image.
- **Outputs:** text/JSON in Postgres; binary exports (CSV/ePub/PDF) generated on demand later.

---

## 11. Error handling & status

- Every `page`/`ocr_result`/`output` carries a status; incremental per-page means partial
  progress survives a failure; per-page **retry** in the UI.
- **`msi` offline / Tailscale down:** `/health` reports Ollama down; OCR/transform can be re-run
  later; captured pages/images are never lost.
- Upload failures retry; oversized/invalid files rejected with clear errors.

---

## 12. Testing

- **Backend:** Go integration tests with **testcontainers-Postgres** (pasar pattern); unit-test
  the template engine's deterministic parts (prompt build, output parse/validate) with a
  **mocked `AIClient`** — no GPU in CI, exactly like invoice-extractor.
- **Mobile:** smoke/widget tests for capture→OCR and reader (later; minimal in v1).
- **CI:** GitHub Actions — `go vet`/`go test`; Flutter analyze/test.

---

## 13. Directory layout

**Backend (`backend/`)**
```
cmd/server/main.go                 # composition root: config → adapters → services → http
internal/
  config/                          # env (OLLAMA_HOST, model tags, BLOB_DIR, JWT, DB)
  domain/                          # entities + PORT interfaces only (zero infra deps)
  app/{document,ocr,transform,template}/   # use-cases (orchestration)
  adapter/
    httpapi/{handler,middleware,router.go} # driving adapter (Fiber)
    store/{sqlc,queries,migrations}        # pgx + sqlc (generated; do not edit sqlc/)
    ai/{client.go,prompts,parse}           # Ollama AIClient (GLM-OCR + qwen2.5); ported prompts/parse
    blob/                                  # filesystem images + thumbnails
  pkg/{jwt,hash}
db/{migrations,queries,seed.sql}   # seed = admin user + the 2 built-in templates
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
**(Not advertised publicly — same caveat as invoice-extractor; add `basic_auth` if exposed.)**

---

## 16. Future (post-v1)

User-defined templates; more template families (filled forms, CSV/XLSX, ePub/PDF export); the
engine/app split (extract a stateless transform service); offline-first capture + sync;
batch/whole-document OCR options; better crop/deskew; multi-user.

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

---

## 18. Open questions

- Structured-template UX: does the user supply field names per run, or pick a preset schema? (v1
  ships one preset; revisit when user-defined templates land.)
- Output export formats priority for v2 (CSV vs ePub vs PDF).
