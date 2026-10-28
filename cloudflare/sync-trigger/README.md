# sync-trigger

Cloudflare Worker (plan gratuito) que lanza `sync-bluesky.yml` cada 5 minutos
mediante `workflow_dispatch`, porque el `schedule` de GitHub se retrasa horas.
El cron del workflow se mantiene como respaldo.

## Despliegue

1. Crea un token fine-grained en GitHub (Settings → Developer settings) limitado
   al repo `Mundo-Dolphins/undolfan` con permiso **Actions: Read and write**.
2. Desde este directorio:

   ```sh
   npx wrangler login
   npx wrangler secret put GITHUB_TOKEN     # el token del paso 1
   npx wrangler secret put TRIGGER_SECRET   # opcional, para disparo manual por HTTP
   npx wrangler deploy
   ```

3. Comprueba en Actions que aparecen runs con evento `workflow_dispatch` cada 5 min.

## Despliegue automático

`.github/workflows/deploy-sync-trigger.yml` hace `wrangler deploy` en cada push a
`main` que toque este directorio (o a mano con `workflow_dispatch`). Requiere los
secrets del repo:

- `CLOUDFLARE_API_TOKEN`: token con la plantilla "Edit Cloudflare Workers".
- `CLOUDFLARE_ACCOUNT_ID`: visible en el dashboard de Cloudflare (Workers & Pages).

`GITHUB_TOKEN` y `TRIGGER_SECRET` son secrets del propio Worker: se guardan una vez
con `wrangler secret put` y los despliegues posteriores los conservan.
