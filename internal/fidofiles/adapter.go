// Package fidofiles bridges *files.Store and fido.FileArea without an
// import cycle (fido cannot import files; files imports config; config imports fido).
package fidofiles

import (
	"os"
	"time"

	"github.com/virtbbs/virtbbs/internal/files"
	"github.com/virtbbs/virtbbs/internal/fido"
)

type storeAdapter struct {
	*files.Store
}

// Adapt returns a FileArea backed by s.
func Adapt(s *files.Store) fido.FileArea {
	if s == nil {
		return nil
	}
	return &storeAdapter{Store: s}
}

func (a *storeAdapter) ListAreaFiles(dirID int64) ([]fido.AreaFile, error) {
	catalog, err := a.Store.ListFiles(dirID)
	if err != nil {
		return nil, err
	}
	out := make([]fido.AreaFile, 0, len(catalog))
	for _, f := range catalog {
		if f == nil {
			continue
		}
		path := a.Store.AbsPath(dirID, f.Filename)
		mod := time.Time{}
		if info, err := os.Stat(path); err == nil {
			mod = info.ModTime()
		}
		out = append(out, fido.AreaFile{
			Filename: f.Filename,
			FullPath: path,
			ModTime:  mod,
			Uploader: f.Uploader,
		})
	}
	return out, nil
}

func (a *storeAdapter) freqListDirs() ([]fido.FreqDirInfo, error) {
	dirs, err := a.Store.ListDirs()
	if err != nil {
		return nil, err
	}
	out := make([]fido.FreqDirInfo, 0, len(dirs))
	for _, d := range dirs {
		if d == nil {
			continue
		}
		out = append(out, fido.FreqDirInfo{ID: d.ID, Name: d.Name, RelPath: d.Path})
	}
	return out, nil
}

func (a *storeAdapter) freqListFiles(dirID int64) ([]fido.FreqFileInfo, error) {
	catalog, err := a.Store.ListFiles(dirID)
	if err != nil {
		return nil, err
	}
	out := make([]fido.FreqFileInfo, 0, len(catalog))
	for _, f := range catalog {
		if f == nil {
			continue
		}
		out = append(out, fido.FreqFileInfo{
			Filename:    f.Filename,
			Size:        f.Size,
			Description: f.Description,
		})
	}
	return out, nil
}

// FreqCatalog returns FREQ catalog access when fa was created by Adapt.
func FreqCatalog(fa fido.FileArea) fido.FreqCatalog {
	if a, ok := fa.(*storeAdapter); ok {
		return a
	}
	return nil
}

func (a *storeAdapter) ListFreqDirs() ([]fido.FreqDirInfo, error) {
	return a.freqListDirs()
}

func (a *storeAdapter) ListFreqFiles(dirID int64) ([]fido.FreqFileInfo, error) {
	return a.freqListFiles(dirID)
}
