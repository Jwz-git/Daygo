package domain

import "encoding/json"

// TrainingPart is an immutable snapshot of an application-level model input.
// Image bytes are local-only until an explicitly approved export or teacher call.
type TrainingPart struct {
	Text   string `json:"text,omitempty"`
	MIME   string `json:"mime,omitempty"`
	Image  []byte `json:"image,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type TrainingSample struct {
	Teachers        []TrainingTeacher `json:"teachers"`
	Version         int               `json:"version"`
	ID              string            `json:"id"`
	Session         string            `json:"session"`
	Group           string            `json:"group"`
	CreatedAt       int64             `json:"createdAt"`
	Purpose         string            `json:"purpose"`
	CodeVersion     string            `json:"codeVersion"`
	Provider        string            `json:"provider"`
	RequestedModel  string            `json:"requestedModel"`
	ActualModel     string            `json:"actualModel"`
	Protocol        string            `json:"protocol"`
	Context         TrainingContext   `json:"context"`
	Parts           []TrainingPart    `json:"parts"`
	Schema          json.RawMessage   `json:"schema"`
	PromptHash      string            `json:"promptHash"`
	SchemaHash      string            `json:"schemaHash"`
	MaxImages       int               `json:"maxImages"`
	MaxOutputTokens int               `json:"maxOutputTokens"`
	Candidate       string            `json:"candidate"`
	Label           string            `json:"label"`
	PrivacyApproved bool              `json:"privacyApproved"`
	Verdict         string            `json:"verdict"`
	Revision        int               `json:"revision"`
}

type TrainingSummary struct {
	ID              string `json:"id"`
	Purpose         string `json:"purpose"`
	CreatedAt       int64  `json:"createdAt"`
	Model           string `json:"model"`
	Verdict         string `json:"verdict"`
	PrivacyApproved bool   `json:"privacyApproved"`
	Revision        int    `json:"revision"`
}

// TrainingContext freezes validation inputs without exposing filesystem paths.
type TrainingContext struct {
	Start      int64    `json:"start"`
	End        int64    `json:"end"`
	Zone       string   `json:"zone"`
	Language   string   `json:"language"`
	Mode       string   `json:"mode"`
	Categories []string `json:"categories"`
}

type TrainingTeacher struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	ActualModel string `json:"actualModel"`
	Protocol    string `json:"protocol"`
	Answer      string `json:"answer"`
	CreatedAt   int64  `json:"createdAt"`
}
