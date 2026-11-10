package flightbyte

// CountryRef is a country as it appears inside an airport.
type CountryRef struct {
	// Code is the ISO 3166-1 alpha-2 code, for example "TR".
	Code string `json:"code"`
	// Name is the country name.
	Name string `json:"name"`
	// Emoji is the flag emoji.
	Emoji string `json:"emoji"`
	// FlagSVG is the absolute URL of the 4:3 SVG flag.
	FlagSVG string `json:"flagSvg"`
	// FlagSVGSquare is the absolute URL of the 1:1 SVG flag.
	FlagSVGSquare string `json:"flagSvgSquare"`
}

// Country is a country from the /countries endpoints.
type Country struct {
	CountryRef
	// Alpha2 is the ISO 3166-1 alpha-2 code.
	Alpha2 string `json:"alpha2"`
	// HasFlag reports whether a flag image exists for the country.
	HasFlag bool `json:"hasFlag"`
}
