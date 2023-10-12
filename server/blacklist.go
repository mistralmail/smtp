package server

// Interface for handling blaclists
// it is meant to be replaced by your own implementation
type Blacklist interface {
	// CheckIp will return true if the IP is blacklisted and false if the IP was not found in a blacklist
	CheckIp(ip string) bool
}
