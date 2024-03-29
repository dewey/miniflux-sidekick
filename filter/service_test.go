package filter

import (
	"github.com/dewey/miniflux-sidekick/rules"
	"github.com/go-kit/kit/log"
	miniflux "miniflux.app/client"
	"testing"
)

func TestEvaluateRules(t *testing.T) {
	type mockService struct {
		rules  []rules.Rule
		l      log.Logger
	}

	tests := []struct {
		name   string
		rules []rules.Rule
		args   *miniflux.Entry
		want   bool
	}{
		{
			name: "Entry contains string",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: "title # Moon",
				},
			},
			args: &miniflux.Entry{
				Title: "Moon entry",
			},
			want: true,
		},
		{
			name: "Entry contains string",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: "title # Moon",
				},
			},
			args: &miniflux.Entry{
				Title: "Sun entry",
			},
			want: false,
		},
		{
			name: "Entry contains string, matched with Regexp",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: "title =~ [Sponsor]",
				},
			},
			args: &miniflux.Entry{
				Title: "[Sponsor] Sun entry",
			},
			want: true,
		},
		{
			name: "Entry doesn't string, matched with Regexp",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: `title =~ \[Sponsor\]`,
				},
			},
			args: &miniflux.Entry{
				Title: "[SponSomethingElsesor] Sun entry",
			},
			want: false,
		},
		{
			name: "Entry doesn't string, matched with Regexp, ignore case",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: "title =~ (?i)(Podcast|scooter)",
				},
			},
			args: &miniflux.Entry{
				Title: "podcast",
			},
			want: true,
		},
		{
			name: "Entry doesn't string, matched with Regexp, ignore case",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: "title =~ (?i)(Podcast|scooter)",
				},
			},
			args: &miniflux.Entry{
				Title: "SCOOTER",
			},
			want: true,
		},
		{
			name: "Entry doesn't string, matched with Regexp, respect case",
			rules: []rules.Rule{
				{
					Command: "ignore-article",
					URL: "http://example.com/feed.xml",
					FilterExpression: "title =~ (Podcast)",
				},
			},
			args: &miniflux.Entry{
				Title: "podcast",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localMockRepository, err := rules.NewLocalRepository()
			if err != nil {
				t.Fatal(err)
			}

			s := service{
				rulesRepository:  localMockRepository,
			}
			s.rulesRepository.SetCachedRules(tt.rules)
			if got := s.evaluateRules(tt.args); got != tt.want {
				t.Errorf("evaluateRules() = %v, want %v", got, tt.want)
			}
		})
	}
}