package rules

import (
	"bufio"
	"os"
)

type localRepository struct {
}

// NewLocalRepository returns a newly initialized rules repository
func NewLocalRepository() (Repository, error) {
	return &localRepository{}, nil
}

// Rules parses a local killfile to get all rules
func (r *localRepository) Rules(location string) ([]Rule, error) {
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
