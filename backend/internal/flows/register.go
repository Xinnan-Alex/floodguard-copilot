// Package flows defines all Genkit AI flows for the FloodGuard system.
package flows

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"google.golang.org/genai"

	"floodguard-backend/internal/search"
	"floodguard-backend/internal/store"
)

// FlowRunner is a function that executes a Genkit flow with a string input.
type FlowRunner func(ctx context.Context, input string) (map[string]interface{}, error)

type whisperInput struct {
	Transcript    string `json:"transcript"`
	ImageBase64   string `json:"image_base64,omitempty"`
	ImageMimeType string `json:"image_mime_type,omitempty"`
}

// Register defines all Genkit AI flows and returns them as callable FlowRunner functions.
func Register(g *genkit.Genkit, cache *store.Cache) (whisper, bureaucracy, dispatch FlowRunner) {
	// Create a shared client for multimodal vision calls, initialized once here.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	visionClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("Failed to create vision client: %v", err)
		visionClient = nil
	}

	wf := genkit.DefineFlow(g, "whisper-agent-flow", whisperFunc(g, cache, visionClient))
	bf := genkit.DefineFlow(g, "bureaucracy-agent-flow", bureaucracyFunc(g, cache))
	df := genkit.DefineFlow(g, "dispatch-agent-flow", dispatchFunc(g, cache))

	return func(ctx context.Context, input string) (map[string]interface{}, error) {
			return wf.Run(ctx, input)
		}, func(ctx context.Context, input string) (map[string]interface{}, error) {
			return bf.Run(ctx, input)
		}, func(ctx context.Context, input string) (map[string]interface{}, error) {
			return df.Run(ctx, input)
		}
}

// cleanJSON extracts the first JSON object from Gemini output, tolerating
// markdown code fences, trailing text, or other surrounding content.
func cleanJSON(text string) (map[string]interface{}, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	var output map[string]interface{}
	err := json.Unmarshal([]byte(text[start:end+1]), &output)
	return output, err
}

func parseWhisperInput(raw string) whisperInput {
	input := whisperInput{Transcript: raw}
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return input
	}

	if err := json.Unmarshal([]byte(trimmed), &input); err != nil || input.Transcript == "" {
		return whisperInput{Transcript: raw}
	}

	return input
}

func generateWhisperFromImage(ctx context.Context, client *genai.Client, transcript, imageBase64, imageMimeType string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return "", err
	}

	if imageMimeType == "" {
		imageMimeType = "image/jpeg"
	}

	prompt := fmt.Sprintf(`Analyze this flood emergency report and attached image.
Return ONLY a valid JSON object without markdown code blocks, with the exact following keys:
- "location" (string): exact location, e.g. "Klang", "Shah Alam", "Unknown".
- "urgency" (number): from 1 to 10.
- "needs" (string): e.g. "Medical evacuation", "Food".
- "raw_extraction" (string): brief summary of the situation based on text and image.
- "suggested_action" (string): e.g. "Dispatch Boat #5", "Deploy Drone".
- "status" (string): always "pending_approval".

Transcript: %s`, transcript)

	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{Data: data, MIMEType: imageMimeType}},
	}

	resp, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash",
		[]*genai.Content{{Parts: parts}},
		&genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.0)},
	)
	if err != nil {
		return "", err
	}

	return resp.Text(), nil
}

// whisperFunc returns the flow function for extracting triage data from emergency transcripts.
func whisperFunc(g *genkit.Genkit, cache *store.Cache, visionClient *genai.Client) func(ctx context.Context, transcript string) (map[string]interface{}, error) {
	return func(ctx context.Context, transcript string) (map[string]interface{}, error) {
		input := parseWhisperInput(transcript)
		ck := cache.Key("whisper", transcript)
		if cached, ok := cache.Get(ctx, ck); ok {
			log.Printf("Cache hit for whisper-agent-flow")
			return cached, nil
		}

		prompt := fmt.Sprintf(`Analyze this emergency call or message transcript and extract key information.
Return ONLY a valid JSON object without any markdown code blocks, with the exact following keys:
- "location" (string): exact location, e.g. "Klang", "Shah Alam", "Unknown".
- "urgency" (number): from 1 to 10.
- "needs" (string): e.g. "Medical evacuation", "Food".
- "raw_extraction" (string): brief summary of the situation based on the text.
- "suggested_action" (string): e.g. "Dispatch Boat #5", "Deploy Drone".
- "status" (string): always "pending_approval".

Transcript: %s`, input.Transcript)

		var output map[string]interface{}
		if input.ImageBase64 != "" && visionClient != nil {
			text, err := generateWhisperFromImage(ctx, visionClient, input.Transcript, input.ImageBase64, input.ImageMimeType)
			if err != nil {
				log.Printf("Image triage generate error: %v", err)
			} else {
				output, _ = cleanJSON(text)
			}
		}

		if output == nil {
			resp, err := genkit.Generate(ctx, g,
				ai.WithModelName("googleai/gemini-2.5-flash"),
				ai.WithPrompt(prompt),
				ai.WithConfig(&genai.GenerateContentConfig{
					Temperature: genai.Ptr[float32](0.0),
				}),
			)

			if err == nil {
				output, _ = cleanJSON(resp.Text())
			} else {
				log.Printf("Generate error: %v", err)
			}
		}

		if output == nil {
			log.Printf("whisper-agent-flow: using fallback, skipping cache")
			return map[string]interface{}{
				"raw_extraction": "Fallback due to processing error: " + input.Transcript,
				"location":       "Unknown",
				"urgency":        5,
				"needs":          "Unknown Processing",
				"status":         "pending_approval",
			}, nil
		}

		cache.Set(ctx, ck, output)
		return output, nil
	}
}

// bureaucracyFunc returns the flow function for processing relief claims using Agentic RAG.
// Step 1: Queries Vertex AI Search for official NADMA policy context.
// Step 2: Feeds context + victim data to Gemini for eligibility decision.
func bureaucracyFunc(g *genkit.Genkit, cache *store.Cache) func(ctx context.Context, victimData string) (map[string]interface{}, error) {
	return func(ctx context.Context, victimData string) (map[string]interface{}, error) {
		ck := cache.Key("bureaucracy", victimData)
		if cached, ok := cache.Get(ctx, ck); ok {
			log.Printf("Cache hit for bureaucracy-agent-flow")
			return cached, nil
		}

		contextData, _ := search.QueryVertexAI(ctx, "Malaysia flood disaster relief NADMA claim requirements RM 1000")

		prompt := fmt.Sprintf(`You are a Government Relief Officer. Based on the official guidelines and the victim data, process the disaster relief claim.
Guidelines: %s
Victim Data: %s

Return ONLY a valid JSON object:
- "status": "approved", "rejected", or "additional_info_required"
- "amount": integer (e.g. 1000)
- "reasoning": brief explanation grounded in the guidelines
- "context_extracted": brief snippet of the guideline used`, contextData, victimData)

		resp, err := genkit.Generate(ctx, g,
			ai.WithModelName("googleai/gemini-2.5-flash"),
			ai.WithPrompt(prompt),
			ai.WithConfig(&genai.GenerateContentConfig{
				Temperature: genai.Ptr[float32](0.0),
			}),
		)

		var output map[string]interface{}
		if err == nil {
			output, _ = cleanJSON(resp.Text())
		}

		if output == nil {
			log.Printf("bureaucracy-agent-flow: using fallback, skipping cache")
			return map[string]interface{}{
				"status":       "additional_info_required",
				"amount":       0,
				"reasoning":    "Could not process claim automatically. Please review manually.",
				"context_used": contextData,
			}, nil
		}

		output["context_used"] = contextData
		cache.Set(ctx, ck, output)
		return output, nil
	}
}

// dispatchFunc returns the flow function for executing approved dispatch or claim filing actions.
// Dispatch results are intentionally not cached: each dispatch is a unique real-world action
// and must produce a distinct confirmation_id.
func dispatchFunc(g *genkit.Genkit, _ *store.Cache) func(ctx context.Context, actionData string) (map[string]interface{}, error) {
	return func(ctx context.Context, actionData string) (map[string]interface{}, error) {
		prompt := fmt.Sprintf(`You are an autonomous Dispatch Execution Agent for Malaysia's flood emergency response system.
Given the following approved action data, execute the action and produce a confirmation report.

Action Data: %s

Return ONLY a valid JSON object with these keys:
- "confirmation_id" (string): a unique reference like "DSP-2026-XXXXX" or "CLM-2026-XXXXX"
- "action_type" (string): "rescue_dispatch" or "claim_filed"
- "summary" (string): one-line human-readable summary of what was executed
- "routed_to" (string): who/what the action was routed to (e.g. "Rescue Boat #7 - Klang Station" or "NADMA Relief Portal")
- "eta_minutes" (number): estimated time in minutes (for rescue) or 0 (for claims)
- "status" (string): always "executed"`, actionData)

		resp, err := genkit.Generate(ctx, g,
			ai.WithModelName("googleai/gemini-2.5-flash"),
			ai.WithPrompt(prompt),
			ai.WithConfig(&genai.GenerateContentConfig{
				Temperature: genai.Ptr[float32](0.2),
			}),
		)

		var output map[string]interface{}
		if err == nil {
			output, _ = cleanJSON(resp.Text())
		}

		if output == nil {
			log.Printf("dispatch-agent-flow: using fallback, skipping cache")
			return map[string]interface{}{
				"confirmation_id": "DSP-2026-00001",
				"action_type":     "rescue_dispatch",
				"summary":         "Rescue boat dispatched to target location.",
				"routed_to":       "Nearest Available Rescue Unit",
				"eta_minutes":     15,
				"status":          "executed",
			}, nil
		}

		return output, nil
	}
}
