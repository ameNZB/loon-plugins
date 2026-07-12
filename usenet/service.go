package usenet

import (
	"context"
	"errors"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

var errNoServer = errors.New("usenet: no server configured")

// service implements pluginapi.UsenetIndex + pluginapi.UsenetAdmin over the
// store + NNTP helpers. One instance is published on the core extension registry
// under both names in the web/all process.
type service struct {
	store        *store
	triggerCrawl func() // set by the plugin in the worker/all process
}

var (
	_ pluginapi.UsenetIndex = (*service)(nil)
	_ pluginapi.UsenetAdmin = (*service)(nil)
)

func (s *service) Search(ctx context.Context, q string, limit int) ([]pluginapi.Release, error) {
	return s.store.searchNzbs(ctx, q, limit)
}

func (s *service) Browse(ctx context.Context, group string, limit int) ([]pluginapi.Release, error) {
	return s.store.browseNzbs(ctx, group, limit)
}

func (s *service) Groups(ctx context.Context) ([]pluginapi.GroupInfo, error) {
	return s.store.groups(ctx)
}

func (s *service) NZB(ctx context.Context, id int64) ([]byte, string, error) {
	raw, filename, err := s.store.nzbData(ctx, id)
	if err != nil {
		return nil, "", err
	}
	data, err := gunzipBytes(raw)
	if err != nil {
		return nil, "", err
	}
	return data, filename, nil
}

func (s *service) Server(ctx context.Context) (pluginapi.Server, error) {
	srv, _, err := s.store.getServer(ctx)
	return srv, err
}

func (s *service) SetServer(ctx context.Context, srv pluginapi.Server) error {
	return s.store.saveServer(ctx, srv)
}

func (s *service) TestConnect(_ context.Context, srv pluginapi.Server) error {
	return testConnect(srv)
}

func (s *service) FetchGroups(ctx context.Context) (int, error) {
	srv, ok, err := s.store.getServer(ctx)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errNoServer
	}
	names, err := listGroups(srv)
	if err != nil {
		return 0, err
	}
	return s.store.upsertGroups(ctx, names)
}

func (s *service) AllGroups(ctx context.Context, query string, limit int) ([]pluginapi.GroupInfo, error) {
	return s.store.allGroups(ctx, query, limit)
}

func (s *service) GroupCount(ctx context.Context) (int, error) {
	return s.store.groupCount(ctx)
}

func (s *service) Stats(ctx context.Context) (pluginapi.IndexStats, error) {
	return s.store.stats(ctx)
}

func (s *service) SetGroupActive(ctx context.Context, name string, active bool) error {
	return s.store.setGroupActive(ctx, name, active)
}

func (s *service) TriggerCrawl() {
	if s.triggerCrawl != nil {
		s.triggerCrawl()
	}
}
