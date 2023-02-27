package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/dewey/miniflux-sidekick/filter"
	"github.com/dewey/miniflux-sidekick/rules"
	"github.com/go-chi/chi"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/peterbourgon/ff"
	"github.com/robfig/cron/v3"
	miniflux "miniflux.app/client"
)

func main() {
	fs := flag.NewFlagSet("mf", flag.ExitOnError)
	var (
		environment          = fs.String("environment", "develop", "the environment we are running in")
		minifluxUsername     = fs.String("username", "", "the username used to log into miniflux")
		minifluxPassword     = fs.String("password", "", "the password used to log into miniflux")
		minifluxAPIKey       = fs.String("api-key", "", "api key used for authentication")
		minifluxAPIEndpoint  = fs.String("api-endpoint", "https://rss.notmyhostna.me", "the api of your miniflux instance")
		killfilePath         = fs.String("killfile-path", "", "the path to the local killfile")
		killfileURL          = fs.String("killfile-url", "", "the url to the remote killfile eg. Github gist")
		killfileRefreshHours = fs.Int("killfile-refresh-hours", 1, "how often the rules should be updated from local or remote config (in hours)")
		refreshInterval      = fs.String("refresh-interval", "", "interval defining how often we check for new entries in miniflux")
		port                 = fs.String("port", "8080", "the port the miniflux sidekick is running on")
		logLevel             = fs.String("log-level", "", "the level to filter logs at eg. debug, info, warn, error")
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
	switch strings.ToLower(*logLevel) {
	case "debug":
		l = level.NewFilter(l, level.AllowDebug())
	case "info":
		l = level.NewFilter(l, level.AllowInfo())
	case "warn":
		l = level.NewFilter(l, level.AllowWarn())
	case "error":
		l = level.NewFilter(l, level.AllowError())
	default:
		switch strings.ToLower(*environment) {
		case "development":
			l = level.NewFilter(l, level.AllowDebug())
		case "prod":
			l = level.NewFilter(l, level.AllowError())
		}
	}
	l = log.With(l, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	var client *miniflux.Client
	if *minifluxUsername != "" && *minifluxPassword != "" {
		client = miniflux.New(*minifluxAPIEndpoint, *minifluxUsername, *minifluxPassword)
	} else if *minifluxAPIKey != "" {
		client = miniflux.New(*minifluxAPIEndpoint, *minifluxAPIKey)
	} else {
		level.Error(l).Log("err", errors.New("api endpoint, username and password or api key need to be provided"))
		return
	}
	u, err := client.Me()
	if err != nil {
		level.Error(l).Log("err", err)
		return
	}
	level.Info(l).Log("msg", "user successfully logged in", "username", u.Username, "user_id", u.ID, "is_admin", u.IsAdmin)

	var t = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	var c = &http.Client{
		Timeout:   time.Second * 10,
		Transport: t,
	}

	// We parse our rules from disk or from an provided endpoint
	var rr rules.Repository
	if *killfilePath != "" {
		level.Info(l).Log("msg", "using a local killfile", "path", *killfilePath)
		localRepo, err := rules.NewLocalRepository()
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		parsedRules, err := localRepo.FetchRules(*killfilePath, l)
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		localRepo.SetCachedRules(parsedRules)
		rr = localRepo
	}
	// A local rule set always trumps a remote one
	if *killfileURL != "" && *killfilePath == "" {
		level.Info(l).Log("msg", "using a remote killfile")
		githubRepo, err := rules.NewGithubRepository(c)
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		parsedRules, err := githubRepo.FetchRules(*killfileURL, l)
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		// Fill cache when fetched first
		githubRepo.SetCachedRules(parsedRules)
		rr = githubRepo

		if *killfileRefreshHours != 0 {
			dur, err := time.ParseDuration(fmt.Sprintf("%dh", *killfileRefreshHours))
			if err != nil {
				level.Error(l).Log("err", err)
				return
			}
			ticker := time.NewTicker(dur)
			go func() {
				for {
					select {
					case <-ticker.C:
						if err := githubRepo.RefreshRules(*killfileURL, l); err != nil {
							level.Error(l).Log("err", err)
						}
					}
				}
			}()
		}
	}

	filterService := filter.NewService(l, client, rr)

	cron := cron.New()
	// Set a fallback, documented in README
	if *refreshInterval == "" {
		*refreshInterval = "*/5 * * * *"
		level.Info(l).Log("msg", "set fallback interval as non provided", "env", *environment, "interval_cron", *refreshInterval)
	}
	switch strings.ToLower(*environment) {
	case "development":
		level.Info(l).Log("msg", "running filter job in simulation mode", "env", *environment, "interval_cron", *refreshInterval)
		filterService.RunFilterJob(true)
	case "prod":
		level.Info(l).Log("msg", "running filter job in destructive mode", "env", *environment, "interval_cron", *refreshInterval)
		_, err := cron.AddJob(*refreshInterval, filterService)
		if err != nil {
			level.Error(l).Log("msg", "error adding cron job to scheduler", "err", err)
		}
		cron.Start()
		for _, e := range cron.Entries() {
			level.Info(l).Log("msg", "cron job entry scheduled", "id", e.ID, "next_execution", e.Next)
		}
	}

	// Set up HTTP API
	r := chi.NewRouter()

	tmpl, err := template.New("rules").Parse(`<html>
	<head>
		<title>miniflux-sidekick</title>
	</head>
	<body style="font-family: monospace;">
	<h1>Currently active rules</h1>
	<table>
	<tr>
		<th>Command</th>
		<th>URL</th>
		<th>Filter Expression</th>
	</tr>
	{{range .}}
		<td>{{ .Command }}</td>
		<td>{{ .URL }}</td>
		<td>{{ .FilterExpression }}</td>
		</tr>
	{{end}}
	</table>
	</body>
	</html>`)
	if err != nil {
		level.Error(l).Log("err", err)
		return
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, rr.Rules())
	})

	level.Info(l).Log("msg", fmt.Sprintf("miniflux-sidekick api is running on :%s", *port), "environment", *environment)

	// Set up webserver and and set max file limit to 50MB
	err = http.ListenAndServe(fmt.Sprintf(":%s", *port), r)
	if err != nil {
		level.Error(l).Log("err", err)
		return
	}
}
