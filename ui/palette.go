package ui

type palette struct {
	HeaderName string
	User       string
	Footer     string
	Error      string
	Info       string
	Activity   string
	TurnSep    string
	Telemetry  string
}

var defaultPalette = palette{
	HeaderName: "1",
	User:       "1",
	Footer:     "",
	Error:      "9",
	Info:       "6",
	Activity:   "",
	TurnSep:    "8",
	Telemetry:  "",
}
