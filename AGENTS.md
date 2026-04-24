# FloodGuard Copilot — Agent Instructions

Hackathon project for Project 2030 (Track 2: Citizens First). An agentic AI dispatch platform for flood rescue coordination. See [README.md](README.md) for full context, architecture diagrams, and setup instructions.

---

## Build & Run

### Backend (Go)
```bash
cd backend
export GEMINI_API_KEY="..."
export GOOGLE_CLOUD_PROJECT="gen-lang-client-0498336364"
go run main.go          # dev server on :3400
go build -o server      # production binary
```

### Frontend (React + TypeScript + Vite)
```bash
cd frontend
npm install
npm run dev             # dev server on :5173
npm run build           # TypeScript check + Vite bundle → dist/
npm run lint            # ESLint
```

### Deploy (Google Cloud Run)
```bash
cd backend  && gcloud run deploy floodguard-backend  --source . --region asia-southeast1 --project gen-lang-client-0498336364
cd frontend && gcloud run deploy floodguard-frontend --source . --region asia-southeast1 --project gen-lang-client-0498336364
```

---

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GEMINI_API_KEY` | *required* | Google AI Studio key |
| `GOOGLE_CLOUD_PROJECT` | `gen-lang-client-0498336364` | GCP project for Firestore + Vertex AI |
| `PORT` | `3400` (local) / `8080` (Cloud Run) | Backend listen port |
| `VERTEX_SEARCH_DATASTORE` | *(optional)* | Discovery Engine datastore ID — falls back to hardcoded NADMA policy |
| `VITE_BACKEND_URL` | `http://localhost:3400` | Frontend → backend URL |

There are no `.env` files. Variables are read via `os.Getenv()` (Go) and `import.meta.env` (Vite).

---

## Architecture

```
frontend/src/
  App.tsx           # State root — polling loop, processItem callbacks
  api.ts            # postJSON<T> helper + VITE_BACKEND_URL
  types.ts          # FeedItem, TriageResult, DispatchResult interfaces
  components/
    FeedPanel.tsx   # Live intake stream + image upload
    TriagePanel.tsx # AI triage display + Google Maps embed
    ActionPanel.tsx # Approve & execute dispatch/claim

backend/
  main.go                        # Firestore init, Genkit init, http.ServeMux + CORS
  internal/
    handler/handler.go           # All 6 HTTP handlers
    flows/register.go            # Genkit flows (whisper, bureaucracy, dispatch)
    models/models.go             # Shared types (FeedItem, request/response structs)
    store/feeds.go               # Firestore feeds collection (add + getsSince)
    store/cache.go               # SHA256-keyed Gemini response cache in Firestore
    search/vertexai.go           # Vertex AI Search / RAG for NADMA policy
    middleware/cors.go           # Permissive CORS for all origins
```

Frontend polls `GET /api/feeds?since={lastID}` every 3 seconds. All other calls are `POST` JSON.

---

## API Endpoints

All defined in [backend/internal/handler/handler.go](backend/internal/handler/handler.go):

| Method | Path | Input body key | Purpose |
|--------|------|----------------|---------|
| `POST` | `/api/whisper` | `transcript` | Extract triage from call transcript |
| `POST` | `/api/triage` | `transcript` | Analyze WhatsApp message/image |
| `POST` | `/api/claim` | `transcript` / `victim_info` | RAG-grounded relief claim filing |
| `POST` | `/api/dispatch` | `action_type, location, urgency, needs, amount, reasoning` | Execute dispatch or claim |
| `POST` | `/api/webhook/whatsapp` | Twilio form data | Ingest WhatsApp messages |
| `GET` | `/api/feeds` | `?since={id}` | Incremental feed polling |

---

## Key Conventions

### Backend (Go)
- Packages are lowercase, single-concern: `handler`, `flows`, `store`, `search`, `middleware`, `models`.
- All Genkit flows follow the same pattern: **check cache → call Gemini → `cleanJSON()` → parse → store cache**. See [flows/register.go](backend/internal/flows/register.go).
- `FlowRunner` type = `func(ctx context.Context, input string) (map[string]interface{}, error)`. Responses are untyped maps; no schema enforcement—rely on Gemini following JSON prompts with fallback defaults.
- Cache key = `SHA256(flowName + ":" + input)`. **Different flows with identical input will collide** — include the flow name in the cache key when adding new flows.
- Gemini temperature: `0.0` for extraction/eligibility flows, `0.2` for dispatch (slight creativity in confirmation text).

### Frontend (TypeScript/React)
- All backend calls go through `postJSON<T>` in [api.ts](frontend/src/api.ts).
- `App.tsx` owns all state. Components receive props + callbacks; they do not call the backend directly (except `ActionPanel` which calls `/api/dispatch` via its own handler passed down).
- Frontend has hardcoded `DEMO_FEEDS` fallback — if backend is offline, the UI still renders demo data. Silent failures by design (no error toasts).
- Local triage results are cached in `triageCacheRef` (a `useRef` map) keyed by feed item ID to avoid re-calling the backend.

---

## Pitfalls

- **No tests exist** (no `*_test.go`, no frontend test suite). Manual testing only.
- **Gemini JSON parsing** is fragile — `cleanJSON()` strips markdown fences. If Gemini changes output format, the fallback JSON objects in each flow activate silently.
- **Cloud Run requires port 8080** — both Dockerfiles handle the `PORT` env var injection automatically. Local dev uses `:3400`.
- **Firestore auth on Cloud Run** uses ADC (Application Default Credentials) — no service account file needed when deployed. Locally, run `gcloud auth application-default login`.
- **CORS is fully permissive** (`*`) — acceptable for a hackathon, restrict before production.
