package instagram

type ProgressType string

const (
	ProgressStory   ProgressType = "STORY"
	ProgressMessage ProgressType = "MESSAGE"
	ProgressMedia   ProgressType = "MEDIA"
)

// ProgressReport is the data packet sent from the Client to the UI
type ProgressReport struct {
	Type       ProgressType
	Step       string
	Current    int // Current segment index (1, 2, 3...)
	Total      int // Total segments count
	Message    string
	BytesSent  int64 // Bytes sent in the CURRENT segment
	TotalBytes int64 // Total bytes of the CURRENT segment
}

// ProgressReporter is the interface used to "send" updates
type ProgressReporter interface {
	Report(report ProgressReport)
}
