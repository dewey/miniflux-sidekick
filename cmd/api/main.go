package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dewey/miniflux-sidekick/filter"
	"github.com/dewey/miniflux-sidekick/rules"
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
		killfilePath        = fs.String("killfile-path", "", "the path to the local killfile")
		killfileURL         = fs.String("killfile-url", "", "the url to the remote killfile eg. Github gist")
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
	var rs []rules.Rule
	fmt.Println((*killfilePath))
	if *killfilePath != "" {
		level.Info(l).Log("msg", "using a local killfile", "path", *killfilePath)
		localRepo, err := rules.NewLocalRepository()
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		parsedRules, err := localRepo.Rules(*killfilePath)
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		rs = parsedRules
	}
	// A local rule set always trumps a remote one
	if *killfileURL != "" && *killfilePath == "" {
		level.Info(l).Log("msg", "using a remote killfile")
		githubRepo, err := rules.NewGithubRepository(c)
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		parsedRules, err := githubRepo.Rules(*killfileURL)
		if err != nil {
			level.Error(l).Log("err", err)
			return
		}
		rs = parsedRules
	}

	filterService := filter.NewService(l, client, rs)

	filterService.RunFilterJob(true)

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
