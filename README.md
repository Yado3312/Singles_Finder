# Closer

Angular frontend with a Go API and optional MongoDB storage. The app currently displays fictional profiles. Account, messaging, and likes persistence are future work; those controls show placeholder messages.

## Run with MongoDB

Requirements: Node.js, npm, Go 1.25 or newer, and Docker with Compose (or your own MongoDB server).

Start local MongoDB:

```sh
docker compose up -d mongo
```

In one terminal, start the Go API. `SEED_DEMO=true` inserts the six fictional profiles **only when the collection is empty**; omit it on later runs.

```sh
cd backend
MONGODB_URI=mongodb://127.0.0.1:27017 MONGODB_DATABASE=singlesfinder SEED_DEMO=true go run .
```

In another terminal, from the repository root:

```sh
npm install
npm start
```

Open `http://localhost:4200`. Angular proxies `/api` to Go at `127.0.0.1:8080`. Search and interest filters call the Go API, which queries MongoDB when `MONGODB_URI` is set. For MongoDB Atlas, set `MONGODB_URI` to your Atlas connection string and keep it on the server; never put it in Angular. A sample configuration is in `backend/.env.example`.

Without `MONGODB_URI`, the API uses fictional in-memory profiles. If the API is offline, Angular displays its local sample profiles and a notice. The map uses OpenStreetMap tiles and needs internet access. Browser geolocation works on localhost or HTTPS.

## API

- `GET /api/health` — process health
- `GET /api/profiles` — discovery list; optional `interest` and `search` query parameters
- `GET /api/profiles/{id}` — one profile

Run backend tests with `cd backend && go test ./...` and build Angular with `npm run build`. Set `ADDR` to change the Go listen address.

## MongoDB collections

The Go server creates indexes on startup for all eight collections (details in `docs/data-model.md`):

- `profiles`: unique string `id`, unique sparse `accountId`, and `interests` indexes. It reads at most 100 profiles per discovery request.
- `private_locations`: unique `accountId`, `2dsphere` index on GeoJSON `point`, and TTL index on `expiresAt`. The app does not write or query private locations yet.

Profile `lat` and `lng` values are **display points for approximate areas**. Do not store a user's precise location in `profiles`; reserve that for `private_locations` and keep it out of API responses. Before real users join, add authentication and authorization, a server-side nearby query that returns coarse locations, rate limits, age controls, blocking, reporting, and moderation. The prototype does not send or store the browser's precise coordinates; “Use my location” only centers the map locally.

The full collection design and indexes are described in [docs/data-model.md](docs/data-model.md). The Atlas URI shared so far contains `<db_username>` and `<db_password>` placeholders, so a live Atlas connection has not been configured. Put the completed URI in the server's `MONGODB_URI` environment variable; do not commit credentials.
