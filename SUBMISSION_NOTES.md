# FloodGuard Copilot Submission Notes

This file contains the architecture summary and judging-alignment notes for submission. It is intentionally stored at the repository root because the `docs/` directory is excluded from version control.

## Project Summary

FloodGuard Copilot is a human-in-the-loop emergency response platform for flood rescue coordination in Malaysia. It helps dispatchers process emergency calls, WhatsApp reports, and post-rescue relief claims faster by combining Gemini-powered extraction, Genkit workflows, Vertex AI Search grounding, and a React dispatch dashboard.

## Architecture Overview

```mermaid
graph TB
    subgraph Users
        Victim[Victim or Reporter]
        Dispatcher[Human Dispatcher]
    end

    subgraph Intake Channels
        Call[Emergency Call Transcript]
        WA[WhatsApp Message or Photo]
        Claim[Shelter Claim Intake]
    end

    subgraph Frontend
        UI[React + TypeScript Dashboard]
        Feed[Live Feed Panel]
        Triage[Triage Panel]
        Action[Action Panel]
        UI --> Feed
        UI --> Triage
        UI --> Action
    end

    subgraph Backend
        API[Go HTTP API]
        Whisper[whisper-agent-flow]
        TriageFlow[triage or vision flow]
        Bureaucracy[bureaucracy-agent-flow]
        Dispatch[dispatch-agent-flow]
        Store[Firestore feeds + cache]
    end

    subgraph Google AI Stack
        Gemini[Gemini 2.5 Flash / Vision]
        Genkit[Firebase Genkit]
        Vertex[Vertex AI Search]
        CloudRun[Cloud Run]
    end

    Victim --> Call
    Victim --> WA
    Victim --> Claim
    Dispatcher --> UI
    Call --> API
    WA --> API
    Claim --> API
    UI --> API

    API --> Whisper
    API --> TriageFlow
    API --> Bureaucracy
    API --> Dispatch

    Whisper --> Genkit
    TriageFlow --> Genkit
    Bureaucracy --> Genkit
    Dispatch --> Genkit

    Genkit --> Gemini
    Bureaucracy --> Vertex
    API --> Store
    UI --> CloudRun
    API --> CloudRun
```

## Operational Flow

```mermaid
sequenceDiagram
    participant Reporter as Victim / Reporter
    participant FE as Frontend Dashboard
    participant BE as Go Backend
    participant GK as Genkit Flow
    participant GM as Gemini
    participant VX as Vertex AI Search
    participant FS as Firestore
    participant Human as Dispatcher

    Reporter->>BE: Call transcript, WhatsApp message, or claim data
    BE->>FS: Store incoming feed item
    FE->>BE: Poll /api/feeds
    BE-->>FE: New feed items
    Human->>FE: Select incident
    FE->>BE: Request triage or claim analysis
    BE->>GK: Run matching workflow
    GK->>GM: Extract location, urgency, needs, reasoning
    alt Claim workflow
        GK->>VX: Retrieve policy context
        VX-->>GK: Eligibility guidance
    end
    GK->>FS: Cache successful result
    BE-->>FE: Structured response
    Human->>FE: Approve dispatch or claim action
    FE->>BE: POST /api/dispatch
    BE->>GK: Run dispatch flow
    GK->>GM: Generate confirmation and routing summary
    BE-->>FE: Confirmation ID, route, ETA
```

## Core Capabilities

### 1. Whisper Agent for Calls

- Parses emergency call transcripts into structured triage data.
- Extracts location, urgency, medical needs, and suggested action.
- Keeps a human dispatcher in control before any dispatch is executed.

### 2. Low-Bandwidth WhatsApp Triage

- Handles text and image-based incident intake.
- Uses Gemini vision reasoning to interpret flood context from uploaded photos.
- Surfaces new incidents into a live dispatch feed for rapid review.

### 3. Zero-Paperwork Relief Claim

- Uses a claim workflow to assess post-disaster aid eligibility.
- Grounds decisions with Vertex AI Search over policy context.
- Reduces administrative friction for victims already affected by flood damage.

## Judging Criteria Alignment

### AI Implementation and Technical Execution

- Uses multiple agentic workflows in Firebase Genkit rather than a single prompt call.
- Combines Gemini extraction, multimodal analysis, dispatch generation, and RAG-supported claim review.
- Persists incoming feeds and successful model outputs through Firestore-backed storage and caching.
- Deploys frontend and backend as production-ready Cloud Run services.

### Innovation and Creativity

- Positions AI as a dispatcher copilot instead of a victim-facing chatbot.
- Balances automation with human approval for high-stakes rescue decisions.
- Unifies rescue logistics and post-disaster claim handling in a single workflow.

### Impact and Problem Relevance

- Directly addresses flood response bottlenecks in Malaysia.
- Supports both immediate rescue coordination and later aid processing.
- Fits the Citizens First theme by reducing operational delay and bureaucratic burden.

### UI/UX and Presentation

- Presents incidents, AI triage, and execution actions in a single operator dashboard.
- Supports rapid review with a live feed, highlighted urgency, and action-oriented summaries.
- Includes demo-friendly flows for call intake, WhatsApp triage, and claim handling.

## Google Stack Alignment

- Gemini: structured extraction, multimodal image reasoning, and dispatch response generation.
- Firebase Genkit: orchestration layer for the agentic workflows.
- Vertex AI Search: retrieval grounding for claim and policy reasoning.
- Firestore: persistent feed store and workflow cache.
- Cloud Run: deployment target for both frontend and backend.