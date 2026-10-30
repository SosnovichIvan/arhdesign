import { performance } from "node:perf_hooks";

const baseUrl = (process.env.LOAD_BASE_URL ?? "http://localhost").replace(/\/$/, "");
const virtualUsers = positiveInteger("LOAD_VUS", process.env.LOAD_VUS ?? "10");
const durationSeconds = positiveInteger("LOAD_DURATION_SECONDS", process.env.LOAD_DURATION_SECONDS ?? "60");
const targetRequestsPerSecond = positiveInteger("LOAD_TARGET_RPS", process.env.LOAD_TARGET_RPS ?? "25");
const username = process.env.LOAD_USERNAME ?? "";
const password = process.env.LOAD_PASSWORD ?? "";
const expectedReadP95 = Number(process.env.LOAD_READ_P95_MS ?? "500");
const expectedWriteP95 = Number(process.env.LOAD_WRITE_P95_MS ?? "800");
const expectedErrorRate = Number(process.env.LOAD_MAX_ERROR_RATE ?? "0.01");
const sharedSession = process.env.LOAD_SHARED_SESSION === "true";

if (!username || !password) throw new Error("LOAD_USERNAME and LOAD_PASSWORD are required");

const deadline = performance.now() + durationSeconds * 1_000;
const readDurations = [];
const writeDurations = [];
let requests = 0;
let errors = 0;

const sharedCookie = sharedSession ? await login(0) : "";
await Promise.all(Array.from({ length: virtualUsers }, (_, index) => runVirtualUser(index, sharedCookie)));

const result = {
  baseUrl,
  durationSeconds,
  errorRate: requests === 0 ? 1 : errors / requests,
  errors,
  readP95Ms: percentile(readDurations, 0.95),
  requests,
  requestsPerSecond: requests / durationSeconds,
  sharedSession,
  targetRequestsPerSecond,
  virtualUsers,
  writeP95Ms: percentile(writeDurations, 0.95),
};
console.log(JSON.stringify(result, null, 2));

if (result.errorRate >= expectedErrorRate || result.readP95Ms > expectedReadP95 || result.writeP95Ms > expectedWriteP95) {
  process.exitCode = 1;
}

async function runVirtualUser(index, existingCookie) {
  const cookie = existingCookie || await login(index);
  while (performance.now() < deadline) {
    await measured("read", "/api/v1/me", { headers: { Cookie: cookie } });
    await pace();
    await measured("read", "/api/v1/projects?pageSize=50", { headers: { Cookie: cookie } });
    await pace();
    await measured("read", "/", {});
    await pace();
  }
}

async function pace() {
  await new Promise((resolve) => setTimeout(resolve, 1_000 * virtualUsers / targetRequestsPerSecond));
}

async function login(index) {
  const started = performance.now();
  const response = await fetch(`${baseUrl}/api/v1/auth/login`, {
    body: JSON.stringify({ identifier: username, password, rememberMe: false }),
    headers: { "Content-Type": "application/json", "Origin": baseUrl, "Sec-Fetch-Site": "same-origin", "X-Load-VU": String(index) },
    method: "POST",
  });
  writeDurations.push(performance.now() - started);
  requests += 1;
  if (!response.ok) {
    errors += 1;
    throw new Error(`Login failed for VU ${index}: HTTP ${response.status}`);
  }
  const values = typeof response.headers.getSetCookie === "function"
    ? response.headers.getSetCookie()
    : [response.headers.get("set-cookie") ?? ""];
  return values.map((value) => value.split(";", 1)[0]).filter(Boolean).join("; ");
}

async function measured(kind, path, options) {
  const started = performance.now();
  try {
    const response = await fetch(`${baseUrl}${path}`, options);
    if (!response.ok) errors += 1;
  } catch {
    errors += 1;
  } finally {
    (kind === "read" ? readDurations : writeDurations).push(performance.now() - started);
    requests += 1;
  }
}

function percentile(values, quantile) {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((left, right) => left - right);
  return Number(sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * quantile) - 1)].toFixed(2));
}

function positiveInteger(name, value) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) throw new Error(`${name} must be a positive integer`);
  return parsed;
}
