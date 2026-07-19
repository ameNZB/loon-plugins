// Package pointstore is a loon plugin: a small points sink that closes the
// economy loop. Users spend the points they earn (e.g. from dailyreward) on a
// profile "flair" — a colored badge shown on their public profile. It composes
// the view system (a /p/store site page + a SlotUserWidget on profiles), the
// points ledger (core.Points.Deduct), and schema-scoped storage.
package pointstore

import (
	"context"
	"embed"
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/the-loon-clan/loon/core"
)

//go:embed migrations/*.sql
var migrations embed.FS

//go:embed templates/*.html
var tmplFS embed.FS

func init() {
	core.RegisterPlugin("pointstore", func() core.Plugin { return &Plugin{} })
}

type Plugin struct {
	core *core.Core
	st   Store
	tmpl *template.Template
}

func (p *Plugin) Metadata() core.Metadata {
	return core.Metadata{
		Name:        "pointstore",
		Version:     "0.1.0",
		Description: "Spend points on a profile flair — the counterpart to points earned elsewhere.",
		Migrations:  migrations,
		Processes:   []string{"web"},
	}
}

func (p *Plugin) Provision(c *core.Core) error {
	p.core = c
	p.st = NewPGStore(c.Storage.SchemaDB("pointstore"))

	t, err := template.ParseFS(tmplFS, "templates/*.html")
	if err != nil {
		return err
	}
	p.tmpl = t

	// The store page (login-gated site page) + its buy action.
	if err := c.RegisterView(core.View{
		Slug: "store", Title: "Store", Slot: core.SlotSitePage,
		MinRole: core.RoleUser,
		Render:  p.renderStore,
		Actions: map[string]func(*gin.Context) (template.HTML, error){
			"buy": p.buy,
		},
	}); err != nil {
		return err
	}

	// The equipped flair, shown on the subject's public profile.
	return c.RegisterView(core.View{
		Slug: "flair", Title: "Flair", Slot: core.SlotUserWidget,
		Public: true,
		Render: p.renderProfileFlair,
	})
}

func (p *Plugin) Start(ctx context.Context) error { return nil }
func (p *Plugin) Stop(ctx context.Context) error  { return nil }
