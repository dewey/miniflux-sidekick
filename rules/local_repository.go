package rules

import (
	"bufio"
	"os"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type localRepository struct {
	mutex       sync.RWMutex
	cachedRules []Rule
}

// NewLocalRepository returns a newly initialized rules repository
func NewLocalRepository() (Repository, error) {
	return &localRepository{}, nil
}

func (r *localRepository) Rules() []Rule {
	if r.cachedRules != nil {
		return r.cachedRules
	} else {
		return []Rule{}
	}
}

// FetchRules parses a local killfile to get all rules
func (r *localRepository) FetchRules(location string, l log.Logger) ([]Rule, error) {
	file, err := os.Open(location)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var rules []Rule
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		matches := reRuleSplitter.FindStringSubmatch(scanner.Text())
		if len(matches) == 4 {
			// Verify that matches[3] (soon to be FilterExpression) is legit before we save the rule
			tokens := filterEntryRegex.FindStringSubmatch(matches[3])
			if tokens == nil || len(tokens) != 4 {
				level.Error(l).Log("err", "invalid filter expression", "expression", matches[3])
			} else {
				rules = append(rules, Rule{
					Command:          matches[1],
					URL:              matches[2],
					FilterExpression: matches[3],
				})
			}
		}
	}
	return rules, scanner.Err()
}

// RefreshRules for local repositories isn't implemented yet.
func (r *localRepository) RefreshRules(location string, l log.Logger) error {
	return nil
}

// SetCachedRules sets the in-memory cache
func (r *localRepository) SetCachedRules(rules []Rule) {
	r.mutex.Lock()
	r.cachedRules = rules
	r.mutex.Unlock()
}
