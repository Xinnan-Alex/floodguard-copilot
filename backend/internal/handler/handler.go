// Package handler provides HTTP handlers for all API endpoints.
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"floodguard-backend/internal/flows"
	"floodguard-backend/internal/models"
	"floodguard-backend/internal/store"
)

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	feedStore   *store.FeedStore
	whisper     flows.FlowRunner
	bureaucracy flows.FlowRunner
	dispatch    flows.FlowRunner
}

// New creates a Handler with all required dependencies.
func New(feedStore *store.FeedStore, whisper, bureaucracy, dispatch flows.FlowRunner) *Handler {
	return &Handler{
		feedStore:   feedStore,
		whisper:     whisper,
		bureaucracy: bureaucracy,
		dispatch:    dispatch,
	}
}

// RegisterRoutes sets up all API routes on an http.ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/whisper", h.Whisper)
	mux.HandleFunc("POST /api/triage", h.Triage)
	mux.HandleFunc("POST /api/claim", h.Claim)
	mux.HandleFunc("POST /api/dispatch", h.Dispatch)
	mux.HandleFunc("POST /api/webhook/whatsapp", h.WhatsAppWebhook)
	mux.HandleFunc("POST /api/webhook/voice", h.VoiceWebhook)
	mux.HandleFunc("GET /api/feeds", h.Feeds)
}

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// errorJSON writes a JSON error response.
func errorJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// randomHex returns n random hex-encoded bytes for use in unique IDs.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// validateTwilioSignature checks the X-Twilio-Signature header against the
// computed HMAC-SHA1 of the request URL + sorted POST params.
// Returns true if authToken is empty (local dev with no token configured).
func validateTwilioSignature(r *http.Request, authToken string) bool {
	if authToken == "" {
		return true
	}
	signature := r.Header.Get("X-Twilio-Signature")
	if signature == "" {
		return false
	}

	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" {
		scheme = "http"
	} else if r.TLS == nil && proto == "" {
		scheme = "http"
	}
	fullURL := scheme + "://" + r.Host + r.URL.RequestURI()

	// Sort POST params alphabetically and append name+value pairs.
	keys := make([]string, 0, len(r.PostForm))
	for k := range r.PostForm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	str := fullURL
	for _, k := range keys {
		str += k + r.PostForm.Get(k)
	}

	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(str))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Whisper handles POST /api/whisper — extract triage data from emergency call transcripts.
func (h *Handler) Whisper(w http.ResponseWriter, r *http.Request) {
	var req models.TranscriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Transcript == "" {
		errorJSON(w, http.StatusBadRequest, "transcript is required")
		return
	}

	result, err := h.whisper(r.Context(), req.Transcript)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Triage handles POST /api/triage — analyze WhatsApp messages and flood images.
func (h *Handler) Triage(w http.ResponseWriter, r *http.Request) {
	var req models.TranscriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Transcript == "" {
		errorJSON(w, http.StatusBadRequest, "transcript is required")
		return
	}

	flowInput := req.Transcript
	if req.ImageBase64 != "" {
		payloadBytes, err := json.Marshal(req)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, "invalid triage payload")
			return
		}
		flowInput = string(payloadBytes)
	}

	result, err := h.whisper(r.Context(), flowInput)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Claim handles POST /api/claim — process disaster relief claims with RAG grounding.
func (h *Handler) Claim(w http.ResponseWriter, r *http.Request) {
	var req models.ClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.bureaucracy(r.Context(), req.Input())
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Dispatch handles POST /api/dispatch — execute approved dispatch or claim filing action.
func (h *Handler) Dispatch(w http.ResponseWriter, r *http.Request) {
	var req models.DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	actionSummary := fmt.Sprintf("Type: %s, Location: %s, Urgency: %d, Needs: %s, Amount: %d, Reasoning: %s",
		req.ActionType, req.Location, req.Urgency, req.Needs, req.Amount, req.Reasoning)

	result, err := h.dispatch(r.Context(), actionSummary)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// WhatsAppWebhook handles POST /api/webhook/whatsapp — receives incoming WhatsApp messages from Twilio.
func (h *Handler) WhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if !validateTwilioSignature(r, authToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body := r.FormValue("Body")
	from := r.FormValue("From")
	mediaURL := r.FormValue("MediaUrl0")
	numMedia := r.FormValue("NumMedia")

	preview := "WhatsApp from " + from
	if body != "" {
		runes := []rune(body)
		if len(runes) > 50 {
			preview = string(runes[:50]) + "..."
		} else {
			preview = body
		}
	}
	if numMedia != "" && numMedia != "0" {
		preview += " [+image]"
	}

	fullData := fmt.Sprintf("WhatsApp message from %s: %s", from, body)
	if mediaURL != "" {
		fullData += fmt.Sprintf("\n[Attached image: %s]", mediaURL)
	}

	item := models.FeedItem{
		ID:        fmt.Sprintf("wa-%d-%s", time.Now().UnixMilli(), randomHex(4)),
		Type:      "whatsapp",
		Timestamp: time.Now().Format("3:04 PM"),
		Preview:   preview,
		FullData:  fullData,
		MediaURL:  mediaURL,
		From:      from,
	}

	h.feedStore.Add(r.Context(), item)
	log.Printf("WhatsApp received from %s: %s", from, body)

	// Auto-triage: run whisper flow in background so feed item arrives pre-triaged.
	if item.FullData != "" {
		go func(feedID, data string) {
			ctx := context.Background()
			result, err := h.whisper(ctx, data)
			if err != nil {
				log.Printf("Auto-triage failed for %s: %v", feedID, err)
				return
			}
			h.feedStore.UpdateTriage(ctx, feedID, result)
			log.Printf("Auto-triage completed for %s", feedID)
		}(item.ID, item.FullData)
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Response></Response>`)
}

// VoiceWebhook handles POST /api/webhook/voice — receives incoming call events from Twilio Voice.
func (h *Handler) VoiceWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if !validateTwilioSignature(r, authToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	from := r.FormValue("From")
	callStatus := r.FormValue("CallStatus")
	callSID := r.FormValue("CallSid")
	speechResult := r.FormValue("SpeechResult")
	recordingURL := r.FormValue("RecordingUrl")

	preview := "Incoming call"
	if from != "" {
		preview = "Incoming call from " + from
	}
	if speechResult != "" {
		runes := []rune(speechResult)
		if len(runes) > 50 {
			preview = string(runes[:50]) + "..."
		} else {
			preview = speechResult
		}
	}

	fullData := fmt.Sprintf("Voice call webhook event\nFrom: %s\nStatus: %s\nCallSid: %s", from, callStatus, callSID)
	if speechResult != "" {
		fullData += "\nTranscript: " + speechResult
	}
	if recordingURL != "" {
		fullData += "\nRecording: " + recordingURL
	}

	item := models.FeedItem{
		ID:        fmt.Sprintf("call-%d-%s", time.Now().UnixMilli(), randomHex(4)),
		Type:      "call",
		Timestamp: time.Now().Format("3:04 PM"),
		Preview:   preview,
		FullData:  fullData,
		From:      from,
	}

	h.feedStore.Add(r.Context(), item)
	log.Printf("Voice webhook received from %s status=%s sid=%s", from, callStatus, callSID)

	// Auto-triage: run whisper flow in background so feed item arrives pre-triaged.
	if item.FullData != "" {
		go func(feedID, data string) {
			ctx := context.Background()
			result, err := h.whisper(ctx, data)
			if err != nil {
				log.Printf("Auto-triage failed for %s: %v", feedID, err)
				return
			}
			h.feedStore.UpdateTriage(ctx, feedID, result)
			log.Printf("Auto-triage completed for %s", feedID)
		}(item.ID, item.FullData)
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Response></Response>`)
}

// Feeds handles GET /api/feeds — returns stored feed items for frontend polling.
func (h *Handler) Feeds(w http.ResponseWriter, r *http.Request) {
	sinceID := r.URL.Query().Get("since")
	result := h.feedStore.GetSince(r.Context(), sinceID)
	writeJSON(w, http.StatusOK, result)
}
