# Un DolFan

Un DolFan convierte publicaciones públicas de Bluesky de `undolfan.mundodolphins.es` en contenido estático permanente con Hugo.

## Arquitectura

Bluesky -> importador Go -> Page Bundles Markdown -> Hugo -> GitHub Actions -> GitHub Pages -> `https://undolfan.mundodolphins.es`

El contenido importado vive en Git bajo `content/posts/YYYY/MM/<slug>/index.md`. Las imágenes se guardan en el mismo Page Bundle para no depender de la CDN de Bluesky tras la importación.

## Requisitos Locales

- Go estable reciente.
- Hugo Extended reciente.
- No se requiere Node.js, base de datos, backend, credenciales de Bluesky ni servicios de pago.

## Comandos

```sh
make test
make lint
make sync
make sync-full
make serve
make build
```

Equivalentes directos del importador:

```sh
go run ./cmd/bluesky-importer sync
go run ./cmd/bluesky-importer sync --full
go run ./cmd/bluesky-importer sync --dry-run
go run ./cmd/bluesky-importer sync --actor undolfan.mundodolphins.es
```

## Hugo

El sitio usa layouts propios en `layouts/` y estilos en `assets/css/main.css`; no depende de themes externos.

Desarrollo local:

```sh
hugo server -D
```

Build:

```sh
hugo --minify
```

## Importación Bluesky

El cliente usa APIs públicas oficiales de AT Protocol:

- `app.bsky.feed.getAuthorFeed` para recorrer el feed del autor.
- `app.bsky.feed.getPostThread` para reconstruir cada conversación.

No hay scraping de HTML ni autenticación. El cliente HTTP está aislado en `internal/bluesky` e incluye timeouts, `context.Context`, reintentos limitados, `User-Agent`, errores tipados y tests con `httptest.Server`.

## Shorts Y Articles

Cada root AT URI produce una única entrada Hugo.

- `content_type: short`: un post propio independiente.
- `content_type: article`: root con al menos `minimum_thread_posts` posts propios conectados. El valor inicial es `2`.

Si un short evoluciona a hilo, se actualiza el mismo Page Bundle, slug y URL. No se crea una segunda entrada.

## Estado

El estado versionado vive en:

```txt
data/bluesky-state.json
```

Guarda la relación estable `root AT URI -> slug` y los posts ya absorbidos por cada root. La primera ejecución hace backfill completo si no hay estado útil. Las sincronizaciones posteriores revisan una ventana reciente para detectar ampliaciones de hilos.

El estado sólo se guarda cuando hay cambios reales, para evitar commits y despliegues innecesarios.

## Recuperación

Si una sincronización falla a mitad, vuelve a ejecutar:

```sh
make sync
```

El estado se actualiza al final de una ejecución correcta. Para reconstruir todo el histórico disponible sin perder slugs ya conocidos:

```sh
make sync-full
```

## Workflows

- `.github/workflows/deploy.yml`: construye Hugo y despliega Pages en push a `main` cuando cambian archivos relevantes o manualmente.
- `.github/workflows/sync-bluesky.yml`: corre cada hora y también con `workflow_dispatch`. Ejecuta tests, sincroniza Bluesky, commitea cambios si existen y despliega Pages dentro del mismo workflow.

El workflow de sync no depende de que un push hecho con `GITHUB_TOKEN` dispare otro workflow.

## GitHub Pages Y Dominio

Configura Pages para desplegar desde GitHub Actions y añade el dominio personalizado:

```txt
undolfan.mundodolphins.es
```

No se incluye `CNAME` porque el dominio puede quedar configurado desde GitHub Pages.

## Estructura

```txt
cmd/bluesky-importer/       CLI de sincronización
internal/bluesky/           cliente HTTP AT Protocol
internal/importer/          algoritmo de backfill, threads e idempotencia
internal/content/           títulos, Markdown y Page Bundles
internal/state/             estado versionado
content/posts/YYYY/MM/      entradas Hugo generadas por mes
layouts/                    plantillas propias
assets/css/                 CSS del sitio
.github/workflows/          deploy y sync
```

## Preservación

El importador no borra artículos históricos sólo porque un post no aparezca temporalmente en una consulta incremental. Incluso si un root deja de estar disponible, el contenido ya importado se conserva salvo que se defina explícitamente otra política.
