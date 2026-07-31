package api

type Result struct {
	Text    string `json:"text"`
	Segment int    `json:"segment"`
	IsFinal bool   `json:"is_final"`
}

type FullResult struct {
	Status int `json:"status"`

	Result *Result `json:"result,omitempty"`

	ID              string    `json:"id,omitempty"`
	OldUpdates      []*Result `json:"old-updates,omitempty"`
	Event           string    `json:"event,omitempty"`
	TranscriptionID string    `json:"transcription-id,omitempty"`
}

type K2Result struct {
	Text          string    `json:"text"`
	Tokens        []string  `json:"tokens"`
	Timestamps    []float64 `json:"timestamps"`
	YsProbs       []float64 `json:"ys_probs"`
	LmProbs       []float64 `json:"lm_probs"`
	ContextScores []float64 `json:"context_scores"`
	Segment       int       `json:"segment"`
	StartTime     float64   `json:"start_time"`
	IsFinal       bool      `json:"is_final"`
	IsEOF         bool      `json:"is_eof"`
}

type EventMsg struct {
	Event string `json:"event,omitempty"`
}

const (
	EventStart     = "START_TRANSCRIPTION"
	EventStartAuto = "START_TRANSCRIPTION_AUTO"
	EventStop      = "STOP_TRANSCRIPTION"
	EventStopping  = "STOPPING_TRANSCRIPTION"
)

type Config struct {
	SkipTour bool `json:"skipTour"`
}

type Part struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Texts struct {
	Parts []Part `json:"parts"`
}
