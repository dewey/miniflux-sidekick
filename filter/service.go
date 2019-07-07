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
}

type service struct {
	rules  []rules.Rule
	client *miniflux.Client
	l      log.Logger
}

// NewService initializes a new filter service
func NewService(l log.Logger, c *miniflux.Client, rules []rules.Rule) Service {
	return &service{
		rules:  rules,
		client: c,
		l:      l,
	}
}

func (s *service) RunFilterJob(simulation bool) {
	// Fetch all feeds.
	f, err := s.client.Feeds()
	if err != nil {
		level.Error(s.l).Log("err", err)
		return
	}
	for _, feed := range f {
		// Check if the feed matches one of our rules
		var found bool
		for _, rule := range s.rules {
			if strings.Contains(feed.FeedURL, rule.URL) {
				found = true
			}
		}
		if !found {
			continue
		}

		// We then get all the unread entries of the feed that matches our rule
		entries, err := s.client.FeedEntries(feed.ID, &miniflux.Filter{
			Status: "unread",
		})
		if err != nil {
			level.Error(s.l).Log("err", err)
			continue
		}

		// We then check if the entry title matches a rule, if it matches we set it to "read" so we don't see it any more
		var matchedEntries []int64
		for _, entry := range entries.Entries {
			var found bool
			for _, rule := range s.rules {
				tokens := strings.Split(rule.FilterExpression, " ")
				if len(tokens) == 3 {
					// We set the string we want to compare against (https://newsboat.org/releases/2.15/docs/newsboat.html#_filter_language are supported in the killfile format)
					var entryTarget string
					switch tokens[0] {
					case "title":
						entryTarget = entry.Title
					case "description":
						entryTarget = entry.Content
					}

					// We check what kind of comparator was given
					switch tokens[1] {
					case "=~":
						matched, err := regexp.MatchString(tokens[2], entryTarget)
						if err != nil {
							level.Error(s.l).Log("err", err)
						}
						if matched {
							found = true
						}
					case "#":
						var containsTerm bool
						blacklistTokens := strings.Split(tokens[2], ",")
						for _, t := range blacklistTokens {
							if strings.Contains(entryTarget, t) {
								containsTerm = true
								break
							}
						}
						if containsTerm {
							found = true
						}
					}
				}
			}
			if found {
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
			if err := s.client.UpdateEntries(matchedEntries, "read"); err != nil {
				level.Error(s.l).Log("msg", "error on updating the feed entries", "err", err)
				return
			}
		}

		level.Info(s.l).Log("msg", "marked all matched feed items as read", "affected", len(matchedEntries))
	}
}
