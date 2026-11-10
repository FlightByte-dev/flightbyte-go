package flightbyte

import (
	"context"
	"net/url"
	"strings"
)

// CountriesService lists country names, emoji and flags. Reach it as
// client.Countries.
type CountriesService struct{ c *Client }

// List returns every country, sorted by name, with names in lang (for
// example "tr"). An empty lang uses the API's default.
func (s *CountriesService) List(ctx context.Context, lang string) ([]Country, error) {
	q := url.Values{}
	setIf(q, "lang", lang)
	var out struct {
		Count int       `json:"count"`
		Data  []Country `json:"data"`
	}
	if err := s.c.do(ctx, "GET", []string{"countries"}, q, nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Get returns one country by its ISO 3166-1 alpha-2 code.
func (s *CountriesService) Get(ctx context.Context, code, lang string) (*Country, error) {
	if err := required("code", code); err != nil {
		return nil, err
	}
	q := url.Values{}
	setIf(q, "lang", lang)
	var country Country
	if err := s.c.do(ctx, "GET", []string{"countries", code}, q, nil, nil, &country); err != nil {
		return nil, err
	}
	return &country, nil
}

// FlagURL returns the URL of a country's SVG flag: 4:3, or 1:1 when square.
// Flags need no API key, so the URL can go straight into an img tag.
func (s *CountriesService) FlagURL(code string, square bool) string {
	var q url.Values
	if square {
		q = url.Values{"shape": {"1x1"}}
	}
	return s.c.endpoint([]string{"flags", strings.ToLower(code) + ".svg"}, q)
}
