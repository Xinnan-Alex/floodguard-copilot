# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**FloodGuard Copilot** — a Human-in-the-Loop AI system for emergency flood response in Malaysia. AI agents (Gemini 2.5 Flash via Firebase Genkit) triage incoming emergency reports and suggest actions; a human dispatcher reviews and approves before any action executes.

## Commands

### Frontend (React + TypeScript + Vite)

```bash
cd frontend
npm install
npm run dev        # Dev server at http://localhost:5173
npm run build      # Type-check + bundle to dist/
npm run lint       # ESLint
npm run preview    # Preview production build
```

### Backend (Go + net/http)

```bash
cd backend
export GEMINI_API_KEY="..."
export GOOGLE_CLOUD_PROJECT="gen-lang-client-0498336364"
go run main.go     # Dev server at :3400
go build -o server # Production binary
```

**There are no automated tests** in either the frontend or backend.

### Environment Variables

| Variable | Default | Required |
|---|---|---|
| `GEMINI_API_KEY` | — | Yes |
| `GOOGLE_CLOUD_PROJECT` | `gen-lang-client-0498336364` | No |
| `PORT` | `3400` (local) / `8080` (Cloud Run) | No |
| `VITE_BACKEND_URL` | `http://localhost:3400` | Frontend only |
| `VERTEX_SEARCH_DATASTORE` | — | No (falls back to hardcoded NADMA policy) |
| `TWILIO_AUTH_TOKEN` | — | No (required only when validating Twilio webhook signatures outside local dev) |

### Deployment (Google Cloud Run)

```bash
cd backend && gcloud run deploy floodguard-backend --source . --region asia-southeast1 --project gen-lang-client-0498336364
cd frontend && gcloud run deploy floodguard-frontend --source . --region asia-southeast1 --project gen-lang-client-0498336364
```

## Architecture

### Request Flow

```
Victim (WhatsApp/Voice/upload) → Backend → Firestore "feeds"
                                                  ↓ (auto-triage runs async)
                              Genkit whisper flow → Gemini → triageResult saved to feed
                                                  ↓
                              Frontend polls GET /api/feeds?since={id} every 3s
                                                  ↓
                              Dispatcher selects item → pre-triaged or manual triage
                                                  ↓
                              Dispatcher reviews AI triage → approves → POST /api/dispatch
```

### Backend (`backend/`)

Pure `net/http` stdlib — no web framework. All handlers in `internal/handler/handler.go` receive injected dependencies (feedStore, flowRunners) via a `Handler` struct.

**Three Genkit AI flows** in `internal/flows/register.go`:
- `whisper-agent-flow` — extracts triage from call transcripts (temp=0.0)
- `bureaucracy-agent-flow` — RAG + Gemini for relief eligibility via Vertex AI Search (temp=0.0)
- `dispatch-agent-flow` — generates dispatch confirmation (temp=0.2)

Each flow follows: **cache lookup → Gemini call → `cleanJSON()` parse → cache store**. Cache key = `SHA256(flowName + ":" + input)` stored in Firestore collection `cache`. This makes flows deterministic and avoids redundant API calls at temp=0.0.

Flow outputs are untyped `map[string]interface{}`; if Gemini returns malformed JSON, each flow silently returns a hardcoded fallback object.

**Persistence** (`internal/store/`):
- `feeds.go` — Firestore collection `feeds`, ordered by `createdAt DESC`, limit 50
- `cache.go` — Firestore collection `cache`, direct doc lookup by SHA256 key

### Frontend (`frontend/src/`)

`App.tsx` owns all state and polling. It passes data down as props and callbacks up — no global state manager.

- `api.ts` — single `postJSON<T>()` helper; `VITE_BACKEND_URL` configures the base URL
- `FeedPanel` — displays live feed, triggers item processing
- `TriagePanel` — shows AI triage results + Google Maps embed
- `ActionPanel` — shows approve/execute UI, calls `/api/dispatch`

**Triage results are cached in a `useRef` map** (`triageCacheRef` in App.tsx) keyed by feed item ID to avoid re-calling the backend when switching between items.

Frontend has hardcoded `DEMO_FEEDS` as a fallback when the backend is unreachable — failures are silent (no error toasts).

### API Endpoints

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/whisper` | Triage from call transcript |
| `POST` | `/api/triage` | Triage from WhatsApp message + optional image (base64) |
| `POST` | `/api/claim` | Relief eligibility check (RAG-grounded) |
| `POST` | `/api/dispatch` | Execute approved dispatch action |
| `POST` | `/api/webhook/whatsapp` | Twilio WhatsApp webhook ingestion + auto-triage |
| `POST` | `/api/webhook/voice` | Twilio Voice call ingestion + auto-triage |
| `GET` | `/api/feeds` | Incremental feed poll (`?since={id}`) |

CORS is fully permissive (`Access-Control-Allow-Origin: *`).

Incoming call events are stored as `call` feed items and, when transcript or speech data is available, are auto-triaged in the background through the same whisper flow used by manual transcript triage.
