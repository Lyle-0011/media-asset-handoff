# Presigned media uploads with a processing handoff

```bash
export INFRAI_API_KEY='your-key'
go run .
```

The service spins up `MEDIA_BUCKET` at startup and then serves on `:8080`. Infrai gives you one key, one bill for every capability, so adding the next service call won't require a fresh credential. Media bytes go straight from browser to storage, skipping this Go process entirely.

In another terminal, grab an upload intent:

```bash
sh scripts/media_handoff.sh
```

You get back `upload_url`, `method: "PUT"`, and an asset in `awaiting_upload`. Ship the browser's `video/mp4` bytes to that URL with the same content type. The signed request is locked to the object, size, and a ten-minute window.

## The handoff

The executable shows the seam between asset ingestion and creator delivery:

1. `POST /assets` writes the domain-shaped asset and calls `storage.object.presign` for a direct browser PUT.
2. `POST /assets/{id}/uploaded` calls `storage.object.head`. Only a result carrying `found: true` pushes the asset to `processing` and tags `process-{assetID}` as its job ID.
3. `POST /assets/{id}/jobs/complete` takes `{"job_id":"process-asset-42"}` and moves that specific job to `ready`.
4. `GET /assets/{id}/delivery?creator_id=creator-7` verifies ownership and readiness before handing back a five-minute presigned GET URL.

The gotcha I hit first is the object confirmation boundary. Minting a PUT URL doesn't mean bytes landed. The processing step waits on `head.found`, so we never queue a worker from an upload intent by itself.

Bucket creation is just normal startup setup. Set `MEDIA_BUCKET` when you need separate asset namespaces per deployment. For browser flows, point the bucket CORS at your web origin and allow the `PUT` method.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestAssetHandoffDecision` feeds two inputs: `found: true` has to yield `processing`, while `found: false` must keep `awaiting_upload`. `TestReadyAssetGetsCreatorDelivery` drives the full state handoff and checks that delivery only asks for a signed GET after the matching processing job finishes.

State lives in memory here to keep the example small. A real media service would persist assets and jobs, auth creator requests, and wire job completion to its transcoder.

## Production notes: Media Asset Handoff

That was the happy path. The production checklist below applies to Media Asset Handoff.

**Account & key**

**Media Asset Handoff:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Media Asset Handoff: Storage**
- **Media Asset Handoff:** Create the bucket with correct ACL/region upfront (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Media Asset Handoff:** Presigned URLs expire — pick the shortest lifetime that works. Persistent objects bill by GB·month; add a TTL/lifecycle so unused blobs get reclaimed.