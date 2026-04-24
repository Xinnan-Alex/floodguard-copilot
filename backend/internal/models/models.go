// Package models defines shared data types used across the application.
package models

// FeedItem represents an incoming message (WhatsApp, call, etc.).
type FeedItem struct {
	ID        string `json:"id" firestore:"id"`
	Type      string `json:"type" firestore:"type"`
	Timestamp string `json:"timestamp" firestore:"timestamp"`
	Preview   string `json:"preview" firestore:"preview"`
	FullData  string `json:"fullData" firestore:"fullData"`
	MediaURL  string `json:"mediaUrl,omitempty" firestore:"mediaUrl,omitempty"`
	From      string `json:"from,omitempty" firestore:"from,omitempty"`
	CreatedAt int64  `json:"createdAt" firestore:"createdAt"`
}

// TranscriptRequest is the request body for /api/whisper and /api/triage.
type TranscriptRequest struct {
	Transcript    string `json:"transcript"`
	ImageBase64   string `json:"image_base64,omitempty"`
	ImageMimeType string `json:"image_mime_type,omitempty"`
}

// ClaimRequest is the request body for /api/claim.
// Accepts both "transcript" and "victim_info" for compatibility.
type ClaimRequest struct {
	Transcript string `json:"transcript"`
	VictimInfo string `json:"victim_info"`
}

// Input returns the best available input string from the claim request.
func (r ClaimRequest) Input() string {
	if r.VictimInfo != "" {
		return r.VictimInfo
	}
	if r.Transcript != "" {
		return r.Transcript
	}
	return "Unknown victim"
}

// DispatchRequest is the request body for /api/dispatch.
type DispatchRequest struct {
	ActionType string `json:"action_type"`
	Location   string `json:"location"`
	Urgency    int    `json:"urgency"`
	Needs      string `json:"needs"`
	Amount     int    `json:"amount"`
	Reasoning  string `json:"reasoning"`
}
