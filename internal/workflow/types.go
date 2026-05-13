package workflow

type ObjectKind string

const (
	ObjectMaster          ObjectKind = "master"
	ObjectEncodeArtifact  ObjectKind = "encode-artifact"
	ObjectProductionAudio ObjectKind = "production-audio"
	ObjectRSS             ObjectKind = "rss"
)

type ObjectState struct {
	Kind        ObjectKind
	Label       string
	Bucket      string
	Key         string
	LocalPath   string
	Local       FileState
	Remote      RemoteState
	Required    bool
	ForceSource bool
}

type FileState struct {
	Exists   bool
	Size     int64
	Checksum string
	ETag     string
}

type RemoteState struct {
	Checked      bool
	Exists       bool
	Size         int64
	Checksum     string
	ETag         string
	ContentType  string
	LastModified string
}

type MetadataState struct {
	Planned   bool
	Applied   bool
	Matches   bool
	Conflicts bool
	Complete  bool
}

type PlanMode string

const (
	ModeFull       PlanMode = "full"
	ModeJustMaster PlanMode = "just-master"
)

type Options struct {
	Mode     PlanMode
	Force    bool
	Yes      bool
	Reencode bool
}

type Decision struct {
	State      State
	Checks     []Check
	Operations []Operation
}

type State string

const (
	StateBlocked          State = "blocked"
	StateNotStarted       State = "not-started"
	StateMetadataApplied  State = "metadata-applied"
	StateMasterSynced     State = "master-synced"
	StateArtifactsReady   State = "artifacts-ready"
	StateEncoded          State = "encoded"
	StateProductionSynced State = "production-synced"
	StateRSSReady         State = "rss-ready"
	StateComplete         State = "complete"
)

type Check struct {
	Kind    string
	Label   string
	Passed  bool
	Reason  string
	Details map[string]string
}

type OperationKind string

const (
	OperationWriteMetadata      OperationKind = "write-metadata"
	OperationUploadMaster       OperationKind = "upload-master"
	OperationDownloadMaster     OperationKind = "download-master"
	OperationUploadArtifact     OperationKind = "upload-artifact"
	OperationDownloadArtifact   OperationKind = "download-artifact"
	OperationEncode             OperationKind = "encode"
	OperationRepairMetadata     OperationKind = "repair-metadata"
	OperationUploadProduction   OperationKind = "upload-production"
	OperationDownloadProduction OperationKind = "download-production"
	OperationRegenerateRSS      OperationKind = "regenerate-rss"
	OperationNoop               OperationKind = "noop"
)

type Operation struct {
	Kind           OperationKind
	ObjectKind     ObjectKind
	Label          string
	Bucket         string
	Key            string
	LocalPath      string
	Reason         string
	RequiresPrompt bool
	DefaultYes     bool
	Destructive    bool
}

func (o Options) normalizedMode() PlanMode {
	if o.Mode == "" {
		return ModeFull
	}
	return o.Mode
}

func (o Options) autoYes() bool {
	return o.Force || o.Yes
}
