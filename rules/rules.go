package rules

import (
	"regexp"
)

var (
	reRuleSplitter = regexp.MustCompile(`(.+?)\s\"?(.+?)\"?\s\"(.+)\"`)
)

// Repository defines the interface for the rules repository
type Repository interface {
	// FetchRules fetches the list of rules from a file or remote location
	FetchRules(location string) ([]Rule, error)

	// RefreshRules refreshes the in-memory cached rules
	RefreshRules(location string) error

	// SetCachedRules([]Rule)
	SetCachedRules(rules []Rule)

	// Rules returns rules from cache
	Rules() []Rule
}

// Rule contains a killfile rule. There's no official standard so we implement these rules https://newsboat.org/releases/2.15/docs/newsboat.html#_killfiles
type Rule struct {
	Command          string
	URL              string
	FilterExpression string
}
