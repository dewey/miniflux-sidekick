package rules

import (
	"bufio"
	"os"
	"sync"
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
func (r *localRepository) FetchRules(location string) ([]Rule, error) {
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
			rules = append(rules, Rule{
				Command:          matches[1],
				URL:              matches[2],
				FilterExpression: matches[3],
			})
		}
	}
	return rules, scanner.Err()
}

// RefreshRules for local repositories isn't implemented yet.
func (r *localRepository) RefreshRules(location string) error {
	return nil
}

// SetCachedRules sets the in-memory cache
func (r *localRepository) SetCachedRules(rules []Rule) {
	r.mutex.Lock()
	r.cachedRules = rules
	r.mutex.Unlock()
}
