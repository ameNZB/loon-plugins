package catalog

import (
	"context"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

// service implements pluginapi.Catalog over the static taxonomy + the disabled
// set. Published on the extension registry so indexer plugins (usenet) can read
// the enabled categories for Newznab caps and categorize releases.
type service struct{ store *store }

var _ pluginapi.Catalog = (*service)(nil)

func (s *service) All(_ context.Context) ([]pluginapi.Category, error) {
	return taxonomy, nil
}

func (s *service) Enabled(ctx context.Context) ([]pluginapi.Category, error) {
	disabled, err := s.store.disabledSet(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]pluginapi.Category, 0, len(taxonomy))
	for _, c := range taxonomy {
		if !disabled[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *service) IsEnabled(ctx context.Context, categoryID int) (bool, error) {
	disabled, err := s.store.disabledSet(ctx)
	if err != nil {
		return false, err
	}
	return !disabled[topLevelOf(categoryID)], nil
}

func (s *service) Categorize(group, title string) int {
	return categorize(group, title)
}

func (s *service) Name(id int) string {
	return categoryName(id)
}
