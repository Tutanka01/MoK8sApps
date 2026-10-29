// miam: vote for tonight's dinner. Votes live in Redis so every pod
// sees the same scoreboard and nothing is lost when pods restart.
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { hostname } from "node:os";
import { createClient } from "redis";

const PORT = Number(process.env.PORT ?? 8080);
const REDIS_URL = process.env.REDIS_URL ?? "redis://localhost:6379";
const KEY = "miam:votes";
const POD = process.env.POD_NAME ?? hostname();

const DISHES = [
  { id: "pizza", name: "Pizza", emoji: "🍕" },
  { id: "tacos", name: "Tacos", emoji: "🌮" },
  { id: "raclette", name: "Raclette", emoji: "🧀" },
  { id: "sushi", name: "Sushi", emoji: "🍣" },
  { id: "couscous", name: "Couscous", emoji: "🥘" },
];

const index = await readFile(new URL("./public/index.html", import.meta.url));

const redis = createClient({ url: REDIS_URL });
redis.on("error", (err) => console.error(`redis: ${err.message}`));
redis.on("ready", () => console.log(`redis: connected to ${REDIS_URL}`));
// Keeps retrying in the background; /api/* answers 503 until it succeeds.
redis.connect().catch((err) => console.error(`redis: ${err.message}`));

function send(res, code, body, type = "application/json") {
  res.writeHead(code, { "Content-Type": type, "Cache-Control": "no-store" });
  res.end(type === "application/json" ? JSON.stringify(body) : body);
}

async function scoreboard() {
  const votes = await redis.hGetAll(KEY);
  const dishes = DISHES.map((d) => ({ ...d, votes: Number(votes[d.id] ?? 0) }));
  const total = dishes.reduce((sum, d) => sum + d.votes, 0);
  return { pod: POD, total, dishes };
}

async function route(req, res) {
  const { pathname } = new URL(req.url, "http://localhost");

  if (req.method === "GET" && pathname === "/") {
    return send(res, 200, index, "text/html; charset=utf-8");
  }
  if (req.method === "GET" && pathname === "/healthz") {
    return send(res, 200, "ok\n", "text/plain");
  }
  // Stays ready while Redis is down on purpose: the page explains the
  // outage instead of the whole Service disappearing.
  if (pathname.startsWith("/api/") && !redis.isReady) {
    return send(res, 503, { pod: POD, error: "Redis ne répond pas" });
  }
  if (req.method === "GET" && pathname === "/api/votes") {
    return send(res, 200, await scoreboard());
  }
  if (req.method === "DELETE" && pathname === "/api/votes") {
    await redis.del(KEY);
    console.log("votes reset");
    return send(res, 200, await scoreboard());
  }
  const match = pathname.match(/^\/api\/votes\/([a-z]+)$/);
  if (req.method === "POST" && match) {
    if (!DISHES.some((d) => d.id === match[1])) {
      return send(res, 404, { pod: POD, error: `Plat inconnu : ${match[1]}` });
    }
    await redis.hIncrBy(KEY, match[1], 1);
    return send(res, 200, await scoreboard());
  }

  send(res, 404, { pod: POD, error: "Rien ici" });
}

const server = createServer((req, res) => {
  route(req, res).catch((err) => {
    console.error(`${req.method} ${req.url}: ${err.message}`);
    if (!res.headersSent) send(res, 500, { pod: POD, error: err.message });
  });
});

server.listen(PORT, () => console.log(`miam listening on :${PORT} (pod ${POD})`));

for (const signal of ["SIGTERM", "SIGINT"]) {
  process.on(signal, () => {
    server.close();
    redis.quit().catch(() => {}).finally(() => process.exit(0));
  });
}
