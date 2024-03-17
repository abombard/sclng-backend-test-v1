package main

import "time"

type GithubEvent struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	Actor struct {
		Id           uint   `json:"id"`
		Login        string `json:"login"`
		DisplayLogin string `json:"display_login"`
		Url          string `json:"url"`
	}
	Repo struct {
		Id   uint   `json:"id"`
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"repo"`
	Payload struct {
		Ref         string `json:"ref"`
		RefType     string `json:"ref_type"`
		Description string `json:"description"`
	} `json:"payload"`
}

type GithubRepository struct {
	Id       uint   `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Id              uint   `json:"id"`
		Login           string `json:"login"`
		Url             string `json:"url"`
		FollowersUrl    string `json:"followers_url"`
		FollowingUrl    string `json:"following_url"`
		OrganizationUrl string `json:"organization_id"`
		ReposUrl        string `json:"repos_url"`
		EventsUrl       string `json:"events_url"`
	} `json:"owner"`
	Fork            bool   `json:"fork"`
	HtmlUrl         string `json:"html_url"`
	Url             string `json:"url"`
	ArchiveUrl      string
	Description     string `json:"description"`
	CommentsUrl     string `json:"comments_url"`
	CommitsUrl      string
	ContentsUrl     string
	ContributorsUrl string
	DownloadsUrl    string
	GitUrl          string
	IssuesUrl       string   `json:"issues_url"`
	Language        string   `json:"language"`
	LanguagesUrl    string   `json:"languages_url"`
	ForksCount      uint     `json:"forks_count"`
	WatchersCount   uint     `json:"watchers_count"`
	Size            uint     `json:"size"`
	OpenIssuesCount uint     `json:"open_issues_count"`
	IsTemplate      bool     `json:"is_template"`
	Topics          []string `json:"topics"`
	HasIssues       bool     `json:"has_issues"`
	HasProjects     bool     `json:"has_projects"`
	HasWiki         bool     `json:"has_wiki"`
	HasPages        bool
	HasDownloads    bool
	HasDiscussion   bool
	Archived        bool
	Disabled        bool
	Visibility      string
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	PushedAt        time.Time `json:"pushed_at"`
	License         struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"license"`
	SubscribersCount uint `json:"subscribers_count"`
	StargazersCount  int  `json:"stargazers_count"`
}
