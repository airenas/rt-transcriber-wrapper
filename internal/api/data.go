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
