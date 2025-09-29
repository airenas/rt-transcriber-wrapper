package domain

type Part struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Texts struct {
	Parts []Part `json:"parts"`
}
