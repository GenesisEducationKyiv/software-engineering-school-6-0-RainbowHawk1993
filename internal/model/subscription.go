package model

import "time"

type Subscription struct {
	ID               int64
	Email            string
	RepoOwner        string
	RepoName         string
	Confirmed        bool
	ConfirmToken     string
	UnsubscribeToken string
	LastSeenTag      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s Subscription) Repo() string {
	return s.RepoOwner + "/" + s.RepoName
}
