package github

import "testing"

func TestParseRemoteRepo(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		owner string
		repo  string
	}{
		{
			name:  "https",
			raw:   "https://github.com/GrexaAI/grexa-app.git",
			owner: "GrexaAI",
			repo:  "grexa-app",
		},
		{
			name:  "ssh scp",
			raw:   "git@github.com:GrexaAI/grexa-app.git",
			owner: "GrexaAI",
			repo:  "grexa-app",
		},
		{
			name:  "ssh url",
			raw:   "ssh://git@github.com/GrexaAI/grexa-app.git",
			owner: "GrexaAI",
			repo:  "grexa-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRemoteRepo(tt.raw)
			if err != nil {
				t.Fatalf("parseRemoteRepo: %v", err)
			}
			if got.Owner != tt.owner || got.Name != tt.repo {
				t.Fatalf("got %s/%s, want %s/%s", got.Owner, got.Name, tt.owner, tt.repo)
			}
		})
	}
}

func TestRepoRefAPIPath(t *testing.T) {
	repo := repoRef{Owner: "GrexaAI", Name: "grexa-app"}
	if got := repo.apiPath("pulls", "703"); got != "repos/GrexaAI/grexa-app/pulls/703" {
		t.Fatalf("apiPath = %q", got)
	}
}
