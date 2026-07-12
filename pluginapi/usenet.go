package pluginapi

import (
	"context"
	"time"
)

// Usenet capability contracts. The usenet plugin publishes these on the core
// extension registry (UsenetIndexName / UsenetAdminName); the host's pages look
// them up so the site queries the indexer without importing the plugin.

// Release is one assembled NZB in search/listing results.
type Release struct {
	ID         int64
	Title      string
	Size       int64
	Posted     time.Time
	Group      string
	Resolution string
	Source     string
	Codec      string
	Audio      string
	Language   string
}

// GroupInfo is one watched newsgroup + how many NZBs it has produced.
type GroupInfo struct {
	Name   string
	Active bool
	NZBs   int64
}

// Server is the NNTP server configuration the setup wizard edits.
type Server struct {
	Host     string
	Port     int
	TLS      bool
	Username string
	Password string
	Enabled  bool
}

// UsenetIndex is the public read surface — registered in the web/all process.
type UsenetIndex interface {
	Search(ctx context.Context, query string, limit int) ([]Release, error)
	Groups(ctx context.Context) ([]GroupInfo, error)
	// NZB returns the decompressed .nzb bytes + a suggested download filename.
	NZB(ctx context.Context, id int64) (data []byte, filename string, err error)
}

// UsenetAdmin is the setup-wizard surface — registered in the web/all process.
type UsenetAdmin interface {
	Server(ctx context.Context) (Server, error)
	SetServer(ctx context.Context, s Server) error
	TestConnect(ctx context.Context, s Server) error               // Dial + Auth + Quit
	FetchGroups(ctx context.Context) (added int, err error)        // NNTP LIST -> insert inactive
	AllGroups(ctx context.Context, limit int) ([]GroupInfo, error) // active-first, for the picker
	SetGroupActive(ctx context.Context, name string, active bool) error
	TriggerCrawl() // fire the crawler job now
}

const (
	UsenetIndexName = "usenet.index"
	UsenetAdminName = "usenet.admin"
)
