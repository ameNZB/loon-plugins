// Package dailyreward is a loon plugin: a home-page card where a signed-in user
// claims a daily reward — once per day, behind a captcha — for points and a
// growing streak. It shows off composing several core seams at once: the view
// system (a SlotSiteWidget card), a plugin route for the claim POST, the points
// ledger (core.Points), schema-scoped storage, and the host's CAPTCHA exposed
// as a cross-cutting capability.
//
// The captcha is looked up structurally off the extension registry (key
// "captcha"), so this plugin never imports the host's captcha package. If no
// host registered one, the claim simply runs without a captcha gate.
package dailyreward

import (
	"context"
	"embed"
	"html/template"

	"github.com/ameNZB/loon/core"
)

//go:embed migrations/*.sql
var migrations embed.FS

//go:embed templates/*.html
var tmplFS embed.FS

// captchaExtension is the registry key the host publishes its captcha under.
// Kept in sync by convention with the host (loon-baseline captcha).
const captchaExtension = "captcha"

func init() {
	core.RegisterPlugin("dailyreward", func() core.Plugin { return &Plugin{} })
}

// captchaCap is the structural view of the host captcha capability — stdlib
// types only, so the plugin needn't import the host's captcha package.
type captchaCap interface {
	Verify(ctx context.Context, token, ip string) error
	WidgetHTML() template.HTML
}

type Plugin struct {
	core    *core.Core
	st      *store
	captcha captchaCap // nil when the host registered none — claim runs ungated
	tmpl    *template.Template
}

func (p *Plugin) Metadata() core.Metadata {
	return core.Metadata{
		Name:        "dailyreward",
		Version:     "0.1.0",
		Description: "Daily-login reward: claim once a day (behind a captcha) for points + a streak.",
		Migrations:  migrations,
		Processes:   []string{"web"},
	}
}

func (p *Plugin) Provision(c *core.Core) error {
	p.core = c
	p.st = &store{db: c.Storage.SchemaDB("dailyreward")}

	if v, ok := c.Lookup(captchaExtension); ok {
		p.captcha, _ = v.(captchaCap)
	}

	t, err := template.ParseFS(tmplFS, "templates/*.html")
	if err != nil {
		return err
	}
	p.tmpl = t

	// SlotSiteWidget ignores Actions, so the claim POST is a plugin route
	// (/plugin/dailyreward/claim) that inherits the host middleware stack.
	c.Router.Mount("dailyreward").POST("/claim", p.claim)

	return c.RegisterView(core.View{
		Slug: "daily-reward", Title: "Daily reward", Slot: core.SlotSiteWidget,
		MinRole: core.RoleUser, // logged-in only
		Render:  p.renderWidget,
	})
}

func (p *Plugin) Start(ctx context.Context) error { return nil }
func (p *Plugin) Stop(ctx context.Context) error  { return nil }
