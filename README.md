# loon-plugins

A collection of [loon](https://github.com/ameNZB/loon) plugins that package the
ameNZB site's background jobs — scrapers, importers, and maintenance tasks — as
self-describing modules instead of a 3,000-line `cmd/main.go` and a 50-service
`pkg/services` directory.

> **Status: scaffold.** Everything here compiles and vets against loon. Two
> pieces:
> - **`scraper/`** — the generic metadata scraper (the chosen direction): one
>   plugin that runs shared jobs over a registry of pluggable
>   `catalog.MetadataSource` modules. The PoC implements the Metadata-Fill job
>   over the existing `catalog.Registry`; sources (`scraper/sources/anidb`, …)
>   land next.
> - **`anidbscraper/`** — an earlier mechanics demo (AniDB as a standalone
>   worker plugin with injected ports + `// EXTRACT:` stubs). Superseded as
>   *packaging* by the scraper module model, but still a valid reference for the
>   host-data worker + `SetDeps` pattern.
>
> The architecture (two axes, the source-module model, the unified
> `catalog_entry` table, the full plugin taxonomy) is in
> [`../Indexer/SCRAPER-ARCHITECTURE.md`](../Indexer/SCRAPER-ARCHITECTURE.md); the
> underlying job inventory + dependency analysis is in
> [`../Indexer/JOBS-AS-PLUGINS.md`](../Indexer/JOBS-AS-PLUGINS.md).

## Why a separate repo

The job machinery (`RegisterJob`, `RunLoop`, off-peak gating, the admin
`/admin/jobs` view) already lives in loon's `schedule` package, so a plugin
inherits the whole scheduler for free. What stays coupled to the host is each
job's **data surface** — the repositories it reads and writes. Packaging a job
as a plugin makes that surface explicit (a small injected interface) instead of
implicit (a field on `*composite.Storage`).

Pulling the jobs into their own module keeps the site binary's plugin list
declarative — add an import, get a job; remove it, lose the job — and lets the
genuinely generic ones (DB maintenance, sitemap, backup) be shared with any
loon site.

## Plugin archetypes

Not every job is the same shape. Three kinds live here (or will):

| Archetype | Owns a schema? | Data access | Example |
|---|---|---|---|
| **Self-contained** | Yes (`Metadata.Migrations`) | `core.Storage.SchemaDB` | (none yet — see `store` in indexer-site) |
| **Host-data worker** | No | narrow ports injected via `SetDeps` | **`anidbscraper`** |
| **Generic/reusable** | Sometimes | mostly `core.Storage.DB()` + config | DB maintenance (planned) |

Most of the site's jobs are **host-data workers**: they operate on tables the
host owns (`nzbs`, `anime_metadata`, `articles`) that other subsystems also
read and write, so the plugin cannot own the schema. `anidbscraper` is the
template for that archetype.

## Layout

```
loon-plugins/
├── go.mod                 module github.com/ameNZB/loon-plugins (replace loon => ../loon)
├── pluginapi/             neutral contracts injected by / consumed from the host
│   ├── scraper.go         CatalogSink (write seam) + Fillable (optional source capability)
│   └── anidb.go           AnimeCatalog, NzbTagSink, TitleMatcher, CoverStore (anidbscraper demo)
├── scraper/               THE generic metadata scraper (chosen direction)
│   ├── plugin.go          owns catalog.Registry (via Lookup); generic Metadata-Fill job
│   ├── deps.go            SetDeps(CatalogSink)
│   └── sources/           source modules land here — each a catalog.MetadataSource
│       └── (anidb, tmdb, mangadex, … — relocated from pkg/services/catalog_sources.go)
└── anidbscraper/          earlier mechanics demo (AniDB as a standalone worker plugin)
    ├── plugin.go          lifecycle + 3 jobs + injected ports
    └── deps.go            Deps + SetDeps
```

## How the site consumes it

1. In `indexer-site/go.mod`:

   ```
   require github.com/ameNZB/loon-plugins v0.0.0-...
   replace github.com/ameNZB/loon-plugins => ../../loon-plugins
   ```

2. In `indexer-site/cmd/main.go`, import the plugin (its `init()` self-registers)
   and inject its host deps in the worker block, before `core.Boot`:

   ```go
   import "github.com/ameNZB/loon-plugins/anidbscraper"

   // ... in the worker/all block, before core.Boot:
   anidbscraper.SetDeps(anidbscraper.Deps{
       Catalog: animeCatalogAdapter{stores.Anime},
       Nzbs:    nzbTagSinkAdapter{stores.Nzb},
       Matcher: anidbService.Matcher(),
       Covers:  coverStoreAdapter{staticDir},
   })
   ```

3. Docker: because the `replace` points outside the build context, the Dockerfile
   pulls the sibling checkout in via a BuildKit named build-context — mirroring
   the existing loon wiring exactly:

   ```
   # push_docker.sh
   docker build --build-context loon=../../loon --build-context loonplugins=../../loon-plugins ...
   # Dockerfile (before `RUN go mod download`)
   COPY --from=loonplugins . /loon-plugins/
   ```

   The relative path in `replace`, the `--build-context` name, and the
   `COPY --from=` destination must all agree — same three-way contract loon
   already uses.

## Development

```
go build ./...
go vet ./...
go test ./...
```

loon is resolved as `../loon`; keep it checked out beside this repo.

## License

MIT — see [LICENSE](LICENSE).
