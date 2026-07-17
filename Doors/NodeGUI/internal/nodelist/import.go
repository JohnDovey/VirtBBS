package nodelist

import (
	"bytes"
	"fmt"
	"time"

	"github.com/JohnDovey/NodeGUI/internal/store"
)

// ImportResult summarizes a completed import.
type ImportResult struct {
	Source    string
	Nodes     int
	LineCount int
	Skipped   int
	NodeDay   int
	Header    string
	Duration  time.Duration
}

// Importer downloads/parses nodelists and writes them to the store.
type Importer struct {
	Store  *store.Store
	Client *Client
	Domain string
}

// ImportRemote downloads the latest daily list from baseURL and replaces the DB.
func (im *Importer) ImportRemote(baseURL string) (*ImportResult, error) {
	start := time.Now()
	if im.Client == nil {
		im.Client = NewClient()
	}
	dl, err := im.Client.FetchLatest(baseURL, time.Now(), 14)
	if err != nil {
		return nil, err
	}
	name, content, err := UnzipNodelist(dl.Data)
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", dl.Filename, err)
	}
	res, err := im.importBytes(dl.URL+" → "+name, content)
	if err != nil {
		return nil, err
	}
	res.Duration = time.Since(start)
	return res, nil
}

// ImportFile loads a local nodelist or zip and replaces the DB.
func (im *Importer) ImportFile(path string) (*ImportResult, error) {
	start := time.Now()
	name, content, err := ReadLocalFile(path)
	if err != nil {
		return nil, err
	}
	res, err := im.importBytes(path+" ("+name+")", content)
	if err != nil {
		return nil, err
	}
	res.Duration = time.Since(start)
	return res, nil
}

func (im *Importer) importBytes(source string, content []byte) (*ImportResult, error) {
	if im.Store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	domain := im.Domain
	if domain == "" {
		domain = "FidoNet"
	}
	doc, err := Parse(bytes.NewReader(content), domain)
	if err != nil {
		return nil, err
	}
	if len(doc.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes parsed from %s", source)
	}
	if err := im.Store.ReplaceAll(doc.Nodes); err != nil {
		return nil, fmt.Errorf("store nodes: %w", err)
	}
	return &ImportResult{
		Source:    source,
		Nodes:     len(doc.Nodes),
		LineCount: doc.LineCount,
		Skipped:   doc.Skipped,
		NodeDay:   doc.HeaderDay,
		Header:    doc.HeaderDate,
	}, nil
}
