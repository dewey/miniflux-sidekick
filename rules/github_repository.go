package rules

import (
	"bufio"
	"net/http"
)

type githubRepository struct {
	c *http.Client
}

// NewGithubRepository returns a newly initialized Github.com repository
func NewGithubRepository(c *http.Client) (Repository, error) {
	return &githubRepository{
		c: c,
	}, nil
}

// Rules parses a local killfile to get all rules
func (r *githubRepository) Rules(location string) ([]Rule, error) {
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
