package filter

import (
	"regexp"
	"strings"

	"github.com/dewey/miniflux-sidekick/rules"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	miniflux "miniflux.app/client"
)

// Service is an interface for a filter service
type Service interface {
	RunFilterJob(simulation bool)
	Run()
}

type service struct {
	rulesRepository  rules.Repository
	client *miniflux.Client
	l      log.Logger
}

// NewService initializes a new filter service
func NewService(l log.Logger, c *miniflux.Client, rr rules.Repository) Service {
	return &service{
		rulesRepository:  rr,
		client: c,
		l:      l,
	}
}

func (s *service) Run() {
	s.RunFilterJob(false)
}

// NOTE (DIRTY HACK): this next var has also been defined in rules/rules.go. If the regex is updated here, it should be updated there as well
var filterEntryRegex = regexp.MustCompile(`(\w+?) (\S+?) (.+)`)

func (s *service) RunFilterJob(simulation bool) {
	// Fetch all feeds.
	f, err := s.client.Feeds()
	if err != nil {
		level.Error(s.l).Log("err", err)
		return
	}

	feedLoop:
	for _, feed := range f {
		// Check if the feed matches one of our rules
		var found bool
		var entries *miniflux.EntryResultSet

		for _, rule := range s.rulesRepository.Rules() {
			// Also support the wildcard selector
			if rule.URL == "*" {
				found = true
			}
			if strings.Contains(feed.FeedURL, rule.URL) {
				found = true
			}
			// Alt: Instead of a URL, specify "category:" followed by a comma-separated list of Miniflux categories to add a rule that affects every feed in those categories.
			if strings.EqualFold(rule.URL[0:9], "category:") {
				categoryTokens := strings.Split(rule.URL[9:], ",")
				for _, ct := range categoryTokens {
					if strings.EqualFold(feed.Category.Title, strings.TrimSpace(ct)) {
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}

			if entries == nil {
				// Get all the unread entries of the feed that matches our rule. Only need to do this once per feed
				entries, err = s.client.FeedEntries(feed.ID, &miniflux.Filter{
					Status: miniflux.EntryStatusUnread,
				})
				if err != nil {
					level.Error(s.l).Log("err", err)
					continue feedLoop // failure to load entries => move to next feed
				}
			}

			// We then check if the entry title matches a rule, if it matches we set it to "read" so we don't see it any more
			var matchedEntries []int64
			for _, entry := range entries.Entries {
				if s.evaluateRule(entry, rule) {
					level.Info(s.l).Log("msg", "entry matches rules in the killfile", "entry_id", entry.ID, "feed_id", feed.ID)
					matchedEntries = append(matchedEntries, entry.ID)
				}
			}

			if simulation {
				for _, me := range matchedEntries {
					e, err := s.client.Entry(me)
					if err != nil {
						level.Error(s.l).Log("err", err)
						return
					}
					level.Info(s.l).Log("msg", "would set status to read", "entry_id", me, "entry_title", e.Title)
				}
			} else {
				for _, me := range matchedEntries {
					level.Info(s.l).Log("msg", "set status to read", "entry_id", me)
					if err := s.client.UpdateEntries([]int64{me}, miniflux.EntryStatusRead); err != nil {
						level.Error(s.l).Log("msg", "error on updating the feed entries", "ids", me, "err", err)
						return
					}
				}
			}
			if len(matchedEntries) > 0 {
				level.Info(s.l).Log("msg", "marked all matched feed items as read", "affected", len(matchedEntries))
			}

		}
	}
}

// evaluateRule checks a feed item against a particular rule. It returns whether this entry should be killed or not.
func (s service) evaluateRule(entry *miniflux.Entry, rule rules.Rule) bool {
	var shouldKill bool

	// The next line should succeed; we tested it would when we loaded our rules
	tokens := filterEntryRegex.FindStringSubmatch(rule.FilterExpression)

	// We set the string we want to compare against (https://newsboat.org/releases/2.15/docs/newsboat.html#_filter_language are supported in the killfile format)
	var entryTarget string
	switch tokens[1] {
	case "title":
		entryTarget = entry.Title
	case "content", "description":
		// include "description" for backwards compatibility with existing killfiles; nobody should be marking entries as read based on the feed's general description
		entryTarget = entry.Content
	case "author":
 		entryTarget = entry.Author
	}

	// We check what kind of comparator was given
	switch tokens[2] {
	case "=~", "!~":
		invertFilter := tokens[2][0] == '!'

		matched, err := regexp.MatchString(tokens[3], entryTarget)
		if err != nil {
			level.Error(s.l).Log("err", err)
		}

		if matched && !invertFilter || !matched && invertFilter {
			shouldKill = true
		}
	case "#", "!#":
		invertFilter := tokens[2][0] == '!'

		var containsTerm bool
		blacklistTokens := strings.Split(tokens[3], ",")
		for _, t := range blacklistTokens {
			if strings.Contains(entryTarget, t) {
				containsTerm = true
				break
			}
		}
		if containsTerm && !invertFilter || !containsTerm && invertFilter {
			shouldKill = true
		}
	}
	return shouldKill
}
