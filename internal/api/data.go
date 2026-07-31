package api

type Hypothesis struct {
	Transcript    string          `json:"transcript"`
	Likelihood    float64         `json:"likelihood"`
	WordAlignment []WordAlignment `json:"word-alignment,omitempty"`
}

type WordAlignment struct {
	Start      float64 `json:"start"`
	Length     float64 `json:"length"`
	Word       string  `json:"word"`
	Confidence float64 `json:"confidence"`
}

type Result struct {
	Hypotheses []Hypothesis `json:"hypotheses"`
	Final      bool         `json:"final"`
}

type ShortResult struct {
	Transcript string `json:"transcript"`
	Segment    int    `json:"segment"`
	Final      bool   `json:"final"`
}

type FullResult struct {
	Status          int            `json:"status"`
	SegmentStart    float64        `json:"segment-start"`
	SegmentLength   float64        `json:"segment-length"`
	TotalLength     float64        `json:"total-length"`
	Result          Result         `json:"result,omitempty"`
	Segment         int            `json:"segment"`
	ID              string         `json:"id,omitempty"`
	OldUpdates      []*ShortResult `json:"old-updates,omitempty"`
	Event           string         `json:"event,omitempty"`
	TranscriptionID string         `json:"transcription-id,omitempty"`
}

type K2Result struct {
	Text          string          `json:"text"`
	Tokens        []string        `json:"tokens"`
	Timestamps    []float64       `json:"timestamps"`
	YsProbs       []float64       `json:"ys_probs"`
	LmProbs       []float64       `json:"lm_probs"`
	ContextScores []float64       `json:"context_scores"`
	Segment       int             `json:"segment"`
	StartTime     float64         `json:"start_time"`
	IsFinal       bool            `json:"is_final"`
	IsEOF         bool            `json:"is_eof"`
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
