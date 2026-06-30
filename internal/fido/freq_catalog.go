package fido

// FreqDirInfo is one file directory for FREQ catalog resolution.
type FreqDirInfo struct {
	ID      int64
	Name    string
	RelPath string
}

// FreqFileInfo is one catalog entry for FREQ listing and matching.
type FreqFileInfo struct {
	Filename    string
	Size        int64
	Description string
}

// FreqCatalog lists file areas for FREQ resolution. Satisfied by fidofiles.Adapt.
type FreqCatalog interface {
	ListFreqDirs() ([]FreqDirInfo, error)
	ListFreqFiles(dirID int64) ([]FreqFileInfo, error)
}
