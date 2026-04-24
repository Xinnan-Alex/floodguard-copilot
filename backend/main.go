// FloodGuard Copilot - Backend Server
// A Human-in-the-Loop Agentic AI for emergency flood response in Malaysia.
// Built with Firebase Genkit (Go SDK), Gemini 2.5 Flash, and Vertex AI Search.
//
// Endpoints:
//
//	POST /api/whisper           - Extract triage data from emergency call transcripts
//	POST /api/triage            - Analyze WhatsApp messages and flood images
//	POST /api/claim             - Process disaster relief claims with RAG grounding
//	POST /api/dispatch          - Execute approved dispatch or claim filing action
//	POST /api/webhook/whatsapp  - Receive incoming WhatsApp messages via Twilio
//	POST /api/webhook/voice     - Receive incoming Twilio Voice call events
//	GET  /api/feeds             - Poll for new feed items (consumed by frontend)
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"floodguard-backend/internal/flows"
	"floodguard-backend/internal/handler"
	"floodguard-backend/internal/middleware"
	"floodguard-backend/internal/store"
)

func main() {
	ctx := context.Background()

	// Initialize Firestore for persistent storage
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "gen-lang-client-0498336364"
	}
	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer fsClient.Close()

	// Initialize stores backed by Firestore
	cache := store.NewCache(fsClient)
	feedStore := store.NewFeedStore(fsClient)

	// Initialize Genkit with Google AI plugin for Gemini model access
	g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{}))

	// Register all AI flows (whisper, bureaucracy, dispatch)
	whisper, bureaucracy, dispatch := flows.Register(g, cache)

	// Create handler with injected dependencies
	h := handler.New(feedStore, whisper, bureaucracy, dispatch)

	// Setup stdlib router with CORS middleware
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3400"
	}

	fmt.Printf("Backend server listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, middleware.CORS(mux)); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
