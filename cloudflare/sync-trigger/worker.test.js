import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import worker from "./worker.js";

const env = {
  GITHUB_REPO: "owner/repo",
  WORKFLOW_FILE: "sync.yml",
  GITHUB_TOKEN: "gh-token",
  TRIGGER_SECRET: "secret",
};

let calls;
let status;
const realFetch = globalThis.fetch;

beforeEach(() => {
  calls = [];
  status = 204;
  globalThis.fetch = async (url, init) => {
    calls.push({ url, init });
    return new Response(status === 204 ? null : "boom", { status });
  };
});

afterEach(() => {
  globalThis.fetch = realFetch;
});

const trigger = (headers = {}, method = "POST") =>
  worker.fetch(new Request("https://worker.test/", { method, headers }), env);

test("scheduled dispatches the workflow on main", async () => {
  const pending = [];
  await worker.scheduled({}, env, { waitUntil: (p) => pending.push(p) });
  await Promise.all(pending);

  assert.equal(calls.length, 1);
  const { url, init } = calls[0];
  assert.equal(url, "https://api.github.com/repos/owner/repo/actions/workflows/sync.yml/dispatches");
  assert.equal(init.method, "POST");
  assert.equal(init.headers.Authorization, "Bearer gh-token");
  assert.deepEqual(JSON.parse(init.body), { ref: "main" });
});

test("fetch without the right secret is a 404 and does not dispatch", async () => {
  assert.equal((await trigger()).status, 404);
  assert.equal((await trigger({ Authorization: "Bearer nope" })).status, 404);
  assert.equal((await trigger({ Authorization: "Bearer secret" }, "GET")).status, 404);
  assert.equal(calls.length, 0);
});

test("fetch with the secret dispatches and returns 202", async () => {
  const res = await trigger({ Authorization: "Bearer secret" });
  assert.equal(res.status, 202);
  assert.equal(calls.length, 1);
});

test("a failed dispatch is reported as 502", async () => {
  status = 401;
  const res = await trigger({ Authorization: "Bearer secret" });
  assert.equal(res.status, 502);
});
