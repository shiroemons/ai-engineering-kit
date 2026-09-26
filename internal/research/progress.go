package research

import "time"

// Progress describes an estimated stage of one research topic.
type Progress struct {
	Percent    int       `json:"percent"`
	Domain     string    `json:"domain,omitempty"`
	DomainName string    `json:"domain_name,omitempty"`
	Topic      string    `json:"topic,omitempty"`
	Phase      string    `json:"phase"`
	UpdatedAt  time.Time `json:"updated_at"`
	Error      string    `json:"error,omitempty"`
}

// ProgressReporter persists progress for a running research batch.
type ProgressReporter func(Progress) error
