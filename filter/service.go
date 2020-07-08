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

func (s *service) Run() {
	s.RunFilterJob(false)
}

var filterEntryRegex = regexp.MustCompile(`(\w+?) (\S+?) (.+)`)

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
			// Also support the wildcard selector
			if rule.URL == "*" {
				found = true
			}
			if strings.Contains(feed.FeedURL, rule.URL) {
				found = true
			}
		}
		if !found {
			continue
		}

		// We then get all the unread entries of the feed that matches our rule
		entries, err := s.client.FeedEntries(feed.ID, &miniflux.Filter{
			Status: miniflux.EntryStatusUnread,
		})
		if err != nil {
			level.Error(s.l).Log("err", err)
			continue
		}

		// We then check if the entry title matches a rule, if it matches we set it to "read" so we don't see it any more
		var matchedEntries []int64
		for _, entry := range entries.Entries {
			var shouldKill bool
			for _, rule := range s.rules {
				tokens := filterEntryRegex.FindStringSubmatch(rule.FilterExpression)
				if tokens == nil || len(tokens) != 4 {
					level.Error(s.l).Log("err", "invalid filter expression", "expression", rule.FilterExpression)
					continue
				}
				// We set the string we want to compare against (https://newsboat.org/releases/2.15/docs/newsboat.html#_filter_language are supported in the killfile format)
				var entryTarget string
				switch tokens[1] {
				case "title":
					entryTarget = entry.Title
				case "description":
					entryTarget = entry.Content
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
			}
			if shouldKill {
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
