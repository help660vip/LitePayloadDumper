package payload

type ProgressEvent struct {
	Partition    string
	TotalOps     int
	CompletedOps int
	BytesDone    uint64
	BytesTotal   uint64
	Stage        string
	Done         bool
	Err          error
}

type ProgressFunc func(ProgressEvent)
