package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/peterbourgon/ff"
	miniflux "miniflux.app/client"
)

func main() {
	fs := flag.NewFlagSet("mfs", flag.ExitOnError)
	var (
		environment         = fs.String("environment", "develop", "the environment we are running in")
		minifluxUsername    = fs.String("username", "dewey", "the username used to log into miniflux")
		minifluxPassword    = fs.String("password", "changeme", "the password used to log into miniflux")
		minifluxAPIEndpoint = fs.String("api-endpoint", "https://rss.notmyhostna.me", "the api of your miniflux instance")
		killfilePath        = fs.String("killfile-path", "./killfile", "the path to the local killfile")
		port                = fs.String("port", "8080", "the port the miniflux sidekick is running on")
	)

	ff.Parse(fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("MF"),
	)

	if *environment == "" {
		panic("environment can't be empty")
	}

	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	switch strings.ToLower(*environment) {
	case "development":
		l = level.NewFilter(l, level.AllowInfo())
	case "prod":
		l = level.NewFilter(l, level.AllowError())
	}
	l = log.With(l, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	if *minifluxAPIEndpoint == "" || *minifluxUsername == "" || *minifluxPassword == "" {
		level.Error(l).Log("err", errors.New("api endpoint, username and password need to be provided"))
		return
	}

	client := miniflux.New(*minifluxAPIEndpoint, *minifluxUsername, *minifluxPassword)
	u, err := client.Me()
	if err != nil {
		level.Error(l).Log("err", err)
		return
	}
	level.Error(l).Log("msg", "user successfully logged in", "username", u.Username, "user_id", u.ID, "is_admin", u.IsAdmin)

	// Fetch all feeds.
	f, err := client.Feeds()
	if err != nil {
		level.Error(l).Log("err", err)
		return
	}

	// We parse our rules from disk or from an provided endpoint
	var rs []Rule
	if *killfilePath != "" {
		parsedRules, err := parseLocalKillfile("./killfile")
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		rs = parsedRules
	}

	for _, feed := range f {
		// Check if the feed matches one of our rules
		var found bool
		for _, rule := range rs {
			if strings.Contains(feed.FeedURL, rule.URL) {
				found = true
			}
		}
		if !found {
			continue
		}

		// We then get all the unread entries of the feed that matches our rule
		entries, err := client.FeedEntries(feed.ID, &miniflux.Filter{
			Status: "unread",
		})
		if err != nil {
			level.Error(l).Log("err", err)
			continue
		}

		// We then check if the entry title matches a rule, if it matches we set it to "read" so we don't see it any more
		var matchedEntries []int64
		for _, entry := range entries.Entries {
			var found bool
			for _, rule := range rs {
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
							level.Error(l).Log("err", err)
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
				level.Info(l).Log("msg", "entry matches rules in the killfile", "entry_id", entry.ID, "feed_id", feed.ID)
				matchedEntries = append(matchedEntries, entry.ID)
			}
		}
		switch *environment {
		case "development", "develop":
			for _, me := range matchedEntries {
				e, err := client.Entry(me)
				if err != nil {
					level.Error(l).Log("err", err)
					return
				}
				level.Info(l).Log("msg", "would set status to read", "entry_id", me, "entry_title", e.Title)
			}
		case "production", "prod":
			if err := client.UpdateEntries(matchedEntries, "read"); err != nil {
				level.Error(l).Log("msg", "error on updating the feed entries", "err", err)
				return
			}
		}

		level.Info(l).Log("msg", "marked all matched feed items as read", "affected", len(matchedEntries))
	}
	// Set up HTTP API
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("miniflux-sidekick"))
	})

	level.Info(l).Log("msg", fmt.Sprintf("miniflux-sidekick api is running on :%s", *port), "environment", *environment)

	// Set up webserver and and set max file limit to 50MB
	err = http.ListenAndServe(fmt.Sprintf(":%s", *port), nil)
	if err != nil {
		level.Error(l).Log("err", err)
		return
	}
}

// Rule contains a killfile rule. There's no official standard so we implement these rules https://newsboat.org/releases/2.15/docs/newsboat.html#_killfiles
type Rule struct {
	Command          string
	URL              string
	FilterExpression string
}

var (
	reRuleSplitter = regexp.MustCompile(`(.+?)\s\"(.+?)\"\s\"(.+)\"`)
)

func parseLocalKillfile(path string) ([]Rule, error) {
	file, err := os.Open(path)
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
