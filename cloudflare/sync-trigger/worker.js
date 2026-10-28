// Dispara el workflow "Sync Bluesky" cada 5 minutos.
// Los `schedule` de GitHub Actions son best-effort y se retrasan horas;
// una llamada a workflow_dispatch arranca el run de inmediato.
export default {
  async scheduled(_event, env, ctx) {
    ctx.waitUntil(dispatch(env));
  },
  // Permite probarlo a mano: curl -X POST <worker-url> -H "Authorization: Bearer $TRIGGER_SECRET"
  async fetch(request, env) {
    if (
      request.method !== "POST" ||
      request.headers.get("Authorization") !== `Bearer ${env.TRIGGER_SECRET}`
    ) {
      return new Response("Not found", { status: 404 });
    }
    return dispatch(env);
  },
};

async function dispatch(env) {
  const res = await fetch(
    `https://api.github.com/repos/${env.GITHUB_REPO}/actions/workflows/${env.WORKFLOW_FILE}/dispatches`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${env.GITHUB_TOKEN}`,
        Accept: "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "undolfan-sync-trigger",
      },
      body: JSON.stringify({ ref: "main" }),
    },
  );
  if (res.status !== 204) {
    const body = await res.text();
    console.error(`dispatch failed: ${res.status} ${body}`);
    return new Response(`dispatch failed: ${res.status}`, { status: 502 });
  }
  return new Response("dispatched", { status: 202 });
}
