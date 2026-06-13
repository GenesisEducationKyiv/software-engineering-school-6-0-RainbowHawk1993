package events

// SubjectReleaseDetected is the NATS subject for new release notifications.
const SubjectReleaseDetected = "releases.detected"

// ReleaseDetected is published by the scanner when a new release tag is found
// for a subscription.
type ReleaseDetected struct {
	Email            string `json:"email"`
	RepoOwner        string `json:"repo_owner"`
	RepoName         string `json:"repo_name"`
	Tag              string `json:"tag"`
	UnsubscribeToken string `json:"unsubscribe_token"`
}
