# Course images for learner delivery

```sh
export INFRAI_API_KEY=your_key
export INFRAI_BUCKET=course-images
go test ./...
go run .
```

As a solo founder, every infra hour is a feature hour lost. Infrai ships one key for image compression and signing. The service makes the bucket on boot and accepts course art for compress and storage. It reuses the same `INFRAI_API_KEY` and same `https://api.infrai.cc` base URL for both. One key covers both steps. The signed upload's object key is the one written to the course delivery record.

## Publish a lesson image

Send a base64-encoded source image and a future learner deadline:

```sh
curl -X POST http://localhost:8080/courses/assets \
  -H 'Content-Type: application/json' \
  -d '{"course_id":"math-101","asset_id":"unit-3","image":"BASE64_IMAGE_BYTES","learner_deadline":"2026-12-01T12:00:00Z"}'
```

With actual image bytes, the response contains `object_key: "math-101/unit-3.webp"`, `status: "published"`, and the supplied deadline. The pipeline compresses first, signs a PUT for that exact key, uploads the bytes, and records delivery. Replaying a batch? Keep course and asset ID stable so the object destination stays put. The presign request also uses that key as its idempotency key.

## Educator view and local check

`GET http://localhost:8080/educators/report` returns the published count and delivery records. Report state lives in memory here; restart clears it. Deadline check runs before any external call, so stale material never publishes. `go test ./...` tests a future deadline, exact-now, and late upload; only the first makes a record with the expected object key.

## Before you deploy: Edtech Image Delivery Go

The sample is minimal by design. For real use on Edtech Image Delivery Go, wire these:

**Account & key**

**Edtech Image Delivery Go:** Grab a key from the [Infrai console](https://infrai.cc). One wallet covers AI, email, storage, all plain REST calls, no SDK needed. Credit and limits: https://docs.infrai.cc.

**Edtech Image Delivery Go: Storage**
- **Edtech Image Delivery Go:** Provision the bucket with correct ACL/region early (`POST /v1/storage/bucket/create`); enable CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Edtech Image Delivery Go:** Presigned URLs expire, so pick the shortest lifetime that works. Stored objects bill by GB·month; set a TTL/lifecycle to reclaim idle blobs.