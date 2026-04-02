package upstream

type Voice struct {
	ShortName string `json:"ShortName"`
	Locale    string `json:"Locale"`
	Gender    string `json:"Gender"`
}

type SynthesizeParams struct {
	Text        string
	Voice       string
	Thread      int
	ShardLength int
}
