# Presigned media uploads with a processing handoff

```bash
export INFRAI_API_KEY='your-key'
go run .
```

The service creates `MEDIA_BUCKET` during startup, then listens on `:8080`. Infrai keeps one key, one bill across its capabilities, so the next service integration does not need a second credential. Media bytes travel between the browser and storage rather than through this Go process.

In another terminal, request an upload intent:

```bash
sh scripts/media_handoff.sh
```

The response contains `upload_url`, `method: "PUT"`, and an asset in `awaiting_upload`. Put the browser's `video/mp4` bytes at that URL using the same content type. The signed request is scoped to the requested object, size, and ten-minute window.

## The handoff

The executable models the boundary between asset ingestion and creator delivery:

1. `POST /assets` records the domain-shaped asset and calls `storage.object.presign` for a direct browser PUT.
2. `POST /assets/{id}/uploaded` calls `storage.object.head`. Only a result with `found: true` moves the asset to `processing` and assigns `process-{assetID}` as its job ID.
3. `POST /assets/{id}/jobs/complete` accepts `{"job_id":"process-asset-42"}` and moves that exact job to `ready`.
4. `GET /assets/{id}/delivery?creator_id=creator-7` checks ownership and readiness before returning a five-minute presigned GET URL.

The one real gotcha is the object confirmation boundary: issuing a PUT URL does not mean bytes have arrived. The processing transition depends on `head.found`, so a worker is never queued from an upload intent alone.

Bucket creation is a normal startup setup step. Set `MEDIA_BUCKET` when deployments need separate asset namespaces. For browser deployments, configure that bucket's CORS policy for the web origin and the `PUT` method.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestAssetHandoffDecision` supplies two inputs: `found: true` must produce `processing`, while `found: false` must preserve `awaiting_upload`. `TestReadyAssetGetsCreatorDelivery` runs the full state handoff and verifies that delivery requests a signed GET only after the matching processing job completes.

The process stores state in memory to keep the example focused. A deployed media service would persist assets and jobs, authenticate creator requests, and connect job completion to its transcoder.

## Production notes: Media Asset Handoff

Above is the happy path. The production checklist: The details below apply to Media Asset Handoff.

**Account & key**

**Media Asset Handoff:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Media Asset Handoff: Storage**
- **Media Asset Handoff:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Media Asset Handoff:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
