package googleclient

// Claims is the trimmed-down view of a Google ID token returned to the
// application service layer.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
	Name          string
	Picture       string
	HostedDomain  string
}
