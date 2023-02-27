package rules

import (
	"bufio"
	"net/http"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
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
func (r *githubRepository) FetchRules(location string, l log.Logger) ([]Rule, error) {
	resp, err := r.c.Get(location)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(resp.Body)
	var rules []Rule
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

// RefreshRules fetches the new rules and updates the local cache
func (r *githubRepository) RefreshRules(location string, l log.Logger) error {
	rules, err := r.FetchRules(location, l)
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
