package rules

import (
	"bufio"
	"net/http"
	"sync"
)

type githubRepository struct {
	c           *http.Client
	mutex       sync.RWMutex
	cachedRules []Rule
}

// NewGithubRepository returns a newly initialized Github.com repository
func NewGithubRepository(c *http.Client) (Repository, error) {
	return &githubRepository{
		c: c,
	}, nil
}

func (r *githubRepository) Rules() []Rule {
	if r.cachedRules != nil {
		return r.cachedRules
	} else {
		return []Rule{}
	}
}

// FetchRules parses a remote killfile to get all rules
func (r *githubRepository) FetchRules(location string) ([]Rule, error) {
	resp, err := r.c.Get(location)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(resp.Body)
	var rules []Rule
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

// RefreshRules fetches the new rules and updates the local cache
func (r *githubRepository) RefreshRules(location string) error {
	rules, err := r.FetchRules(location)
	if err != nil {
		return err
	}
	r.SetCachedRules(rules)
	return nil
}

// SetCachedRules sets the in-memory cache
func (r *githubRepository) SetCachedRules(rules []Rule) {
	r.mutex.Lock()
	r.cachedRules = rules
	r.mutex.Unlock()
}
