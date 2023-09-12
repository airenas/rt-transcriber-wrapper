package api

type Hypothesis struct {
    Transcript string  `json:"transcript"`
    Likelihood float64 `json:"likelihood"`
}

type Result struct {
    Hypotheses []Hypothesis `json:"hypotheses"`
    Final      bool        `json:"final"`
}

type FullResult struct {
    Status        int     `json:"status"`
    SegmentStart  float64 `json:"segment-start"`
    SegmentLength float64 `json:"segment-length"`
    TotalLength   float64 `json:"total-length"`
    Result        Result  `json:"result"`
    Segment       int     `json:"segment"`
    ID            string  `json:"id"`
}