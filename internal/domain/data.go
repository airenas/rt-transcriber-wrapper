package domain

type K2Word struct {
	Text        string
	Punctuated  string
	SentenceEnd bool
	Timestamp   float64
	Segment     int
}

type K2Words struct {
	Words     []*K2Word
	Segment   int
	StartTime float64
}

type K2Data struct {
	Words            []*K2Word
	NewWords         []*K2Word
	StartTime        float64
	FinalTo          int // index of last word that is final in Words
	FinalSegmentID   int
	CurrentSegmentID int

	LastAppendFromIndex int
}
