package events

const SubjectReleaseDetected = "releases.detected"

type ReleaseDetected struct {
	Email            string `json:"email"`
	RepoOwner        string `json:"repo_owner"`
	RepoName         string `json:"repo_name"`
	Tag              string `json:"tag"`
	UnsubscribeToken string `json:"unsubscribe_token"`
}
