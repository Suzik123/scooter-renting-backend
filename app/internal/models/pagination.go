package models

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

type Page struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Clamp returns p with Limit bounded to 1..MaxPageLimit and Offset >= 0.
func (p Page) Clamp() Page {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}
