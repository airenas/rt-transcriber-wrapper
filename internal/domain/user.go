package domain

type User struct {
	ID       string `json:"id"`
	SkipTour bool   `json:"showTour"`
}

