package usenet

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ameNZB/loon/core"
	"github.com/ameNZB/loon/schedule"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

// The plugin owns its admin pages: the setup wizard (settings view) and the
// live crawl/backfill status page (status view). Each Render returns an HTML
// fragment from the embedded templates below; the HOST wraps it in its own
// layout/nav/theme via loon's AdminView seam, so the pages look native to any
// site without the site writing usenet-specific handlers.

//go:embed templates/*.html
var viewFS embed.FS

const (
	settingsURL = "/admin/p/usenet"
	crawlersURL = "/admin/p/crawlers"
)

func (p *Plugin) registerViews(c *core.Core) error {
	t, err := template.ParseFS(viewFS, "templates/*.html")
	if err != nil {
		return err
	}
	p.tmpl = t

	if err := c.RegisterAdminView(core.AdminView{
		Slug: "usenet", Title: "Usenet setup", Kind: core.ViewSettings,
		Render: func(gc *gin.Context) (template.HTML, error) {
			srv, _, _ := p.st.getServer(gc.Request.Context())
			return p.renderSettings(gc.Request.Context(), srv, gc.Query("gq"), gc.Query("msg"), gc.Query("err"))
		},
		Actions: map[string]func(*gin.Context) (template.HTML, error){
			"server":       p.actionSaveServer,
			"test":         p.actionTestServer,
			"fetch-groups": p.actionFetchGroups,
			"group":        p.actionToggleGroup,
			"crawl":        p.actionCrawl(settingsURL),
		},
	}); err != nil {
		return err
	}

	return c.RegisterAdminView(core.AdminView{
		Slug: "crawlers", Title: "Crawlers", Kind: core.ViewStatus,
		Render: func(gc *gin.Context) (template.HTML, error) {
			return p.renderCrawlers(gc.Request.Context(), gc.Query("msg"), gc.Query("err"))
		},
		Actions: map[string]func(*gin.Context) (template.HTML, error){
			"crawl":    p.actionCrawl(crawlersURL),
			"backfill": p.actionBackfill,
			"reset-backfill": func(gc *gin.Context) (template.HTML, error) {
				name := gc.PostForm("name")
				_ = p.st.resetBackfill(gc.Request.Context(), name)
				return redirect(gc, crawlersURL, "msg", "backfill re-armed for "+name)
			},
		},
	})
}

func (p *Plugin) frag(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := p.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// redirect answers the action with a 303 back to the view; the empty fragment
// tells the host the response is already written.
func redirect(gc *gin.Context, base, key, msg string) (template.HTML, error) {
	gc.Redirect(http.StatusSeeOther, base+"?"+key+"="+url.QueryEscape(msg))
	return "", nil
}

// ── settings view ───────────────────────────────────────────────────

func (p *Plugin) renderSettings(ctx context.Context, srv pluginapi.Server, gq, msg, errMsg string) (template.HTML, error) {
	groups, _ := p.st.allGroups(ctx, gq, 300)
	total, _ := p.st.groupCount(ctx)
	return p.frag("settings.html", map[string]any{
		"Server": srv, "Groups": groups, "GroupQuery": gq,
		"GroupTotal": total, "Shown": len(groups),
		"Msg": msg, "Err": errMsg,
	})
}

func formServer(gc *gin.Context) pluginapi.Server {
	port, _ := strconv.Atoi(gc.PostForm("port"))
	if port == 0 {
		port = 119
	}
	tls := gc.PostForm("tls")
	return pluginapi.Server{
		Host:     strings.TrimSpace(gc.PostForm("host")),
		Port:     port,
		TLS:      tls == "on" || tls == "true",
		Username: gc.PostForm("username"),
		Password: gc.PostForm("password"),
		Enabled:  true,
	}
}

func (p *Plugin) actionSaveServer(gc *gin.Context) (template.HTML, error) {
	if err := p.st.saveServer(gc.Request.Context(), formServer(gc)); err != nil {
		return redirect(gc, settingsURL, "err", err.Error())
	}
	return redirect(gc, settingsURL, "msg", "server saved")
}

// actionTestServer re-renders the fragment with the SUBMITTED values (not a
// redirect) so the form keeps everything typed, whatever the result.
func (p *Plugin) actionTestServer(gc *gin.Context) (template.HTML, error) {
	srv := formServer(gc)
	if err := testConnect(srv); err != nil {
		return p.renderSettings(gc.Request.Context(), srv, "", "", "connection failed: "+err.Error())
	}
	return p.renderSettings(gc.Request.Context(), srv, "", "connection ok — click Save to keep it", "")
}

func (p *Plugin) actionFetchGroups(gc *gin.Context) (template.HTML, error) {
	n, err := p.svc.FetchGroups(gc.Request.Context())
	if err != nil {
		return redirect(gc, settingsURL, "err", "fetch failed: "+err.Error())
	}
	return redirect(gc, settingsURL, "msg", "fetched "+strconv.Itoa(n)+" new group(s)")
}

func (p *Plugin) actionToggleGroup(gc *gin.Context) (template.HTML, error) {
	_ = p.st.setGroupActive(gc.Request.Context(), gc.PostForm("name"), gc.PostForm("active") == "true")
	dest := settingsURL
	if gq := gc.PostForm("gq"); gq != "" {
		dest += "?gq=" + url.QueryEscape(gq) // keep the current group search
	}
	gc.Redirect(http.StatusSeeOther, dest)
	return "", nil
}

func (p *Plugin) actionCrawl(backTo string) func(*gin.Context) (template.HTML, error) {
	return func(gc *gin.Context) (template.HTML, error) {
		p.svc.TriggerCrawl()
		return redirect(gc, backTo, "msg", "crawl triggered")
	}
}

func (p *Plugin) actionBackfill(gc *gin.Context) (template.HTML, error) {
	p.svc.TriggerBackfill()
	return redirect(gc, crawlersURL, "msg", "backfill triggered")
}

// ── crawlers (status) view ──────────────────────────────────────────

type crawlerGroupVM struct {
	Name         string
	NZBs, Staged int
	Cover        pluginapi.CoverageBar
	FwdDate      string
	BackDate     string
	BackfillDone bool
	Remaining    int64
	LastCrawl    string
}

type crawlerJobVM struct {
	Name     string
	Status   string
	Activity string
	Running  bool
}

func (p *Plugin) renderCrawlers(ctx context.Context, msg, errMsg string) (template.HTML, error) {
	stats, err := p.st.stats(ctx)
	if err != nil {
		return "", err
	}
	groups := make([]crawlerGroupVM, len(stats.Groups))
	for i, g := range stats.Groups {
		vm := crawlerGroupVM{
			Name: g.Name, NZBs: g.NZBs, Staged: g.Staged,
			Cover: g.Coverage(), BackfillDone: g.BackfillDone,
			FwdDate: fmtDate(g.HighWatermarkDate), BackDate: fmtDate(g.BackWatermarkDate),
			LastCrawl: fmtTime(g.LastCrawl),
		}
		if !g.BackfillDone && g.BackWatermark > g.ServerLow {
			vm.Remaining = g.BackWatermark - g.ServerLow
		}
		groups[i] = vm
	}
	jobs, running := p.jobVMs()
	return p.frag("crawlers.html", map[string]any{
		"Stats": stats, "Groups": groups, "Jobs": jobs,
		"AutoRefresh": running, "Msg": msg, "Err": errMsg,
	})
}

// jobVMs snapshots this plugin's own jobs so the page shows what each is doing.
func (p *Plugin) jobVMs() (jobs []crawlerJobVM, anyRunning bool) {
	mine := map[string]bool{
		"Usenet Crawler": true, "Usenet Backfill": true,
		"NZB Builder": true, "NZB Tag Fill": true, "NZB Prune": true,
	}
	for _, s := range schedule.GetAllSnapshots() {
		if !mine[s.Name] {
			continue
		}
		j := crawlerJobVM{Name: s.Name, Status: s.Status}
		if s.LastError != "" {
			j.Activity = s.LastError
		} else if len(s.Logs) > 0 {
			j.Activity = s.Logs[len(s.Logs)-1]
		}
		if s.Status == "running" || s.ElapsedSecs > 0 {
			j.Running, anyRunning = true, true
		}
		jobs = append(jobs, j)
	}
	return jobs, anyRunning
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02")
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02 15:04:05")
}
