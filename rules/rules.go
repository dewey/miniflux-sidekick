package rules

import (
	"regexp"

	"github.com/go-kit/kit/log"
)

var (
	reRuleSplitter = regexp.MustCompile(`(.+?)\s\"?(.+?)\"?\s\"(.+)\"`)

	// DIRTY HACK: copied from filter/service.go (so we can verify rules are properly formatted while loading them); keep these regexes in sync!
	filterEntryRegex = regexp.MustCompile(`(\w+?) (\S+?) (.+)`)
)

// Repository defines the interface for the rules repository
type Repository interface {
	// FetchRules fetches the list of rules from a file or remote location
	FetchRules(location string, l log.Logger) ([]Rule, error)

	// RefreshRules refreshes the in-memory cached rules
	RefreshRules(location string, l log.Logger) error

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
