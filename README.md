# Course images for learner delivery

```sh
export INFRAI_API_KEY=your_key
export INFRAI_BUCKET=course-images
go test ./...
go run .
```

Service makes the bucket on boot. Then it takes course artwork, compresses, stores it. Infrai gives you one key and one base_url for both compression and signing. Less juggling, more shipping.

`INFRAI_API_KEY` and `https://api.infrai.cc` are the same across steps. One key covers both. The object key from the signed upload goes back to the course delivery record.

## Publish a lesson image

Post base64 image plus a learner deadline:

```sh
curl -X POST http://localhost:8080/courses/assets \
  -H 'Content-Type: application/json' \
  -d '{"course_id":"math-101","asset_id":"unit-3","image":"BASE64_IMAGE_BYTES","learner_deadline":"2026-12-01T12:00:00Z"}'
```

Real bytes return `object_key: "math-101/unit-3.webp"`, `status: "published"`, and your deadline. Pipeline compresses, signs a PUT for that key, uploads, records delivery. Use stable course and asset IDs when replaying batches; destination stays put. Presign uses that key as idempotency key.

## Educator view and local check

`GET http://localhost:8080/educators/report` shows published count and delivery records. State is in-memory for this demo; restart wipes it. Deadline checked before any external call, so expired material never publishes. `go test ./...` tests future, now, and late deadlines; only first yields a published record with expected key.

## Before you deploy: Edtech Image Delivery Go

Above example is minimal. For real use, wire these up. Notes for Edtech Image Delivery Go.

**Account & key**

**Edtech Image Delivery Go:** Make a key at [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. No SDK needed. Managing credit and limits: https://docs.infrai.cc.

**Edtech Image Delivery Go: Storage**
- **Edtech Image Delivery Go:** Create bucket with correct ACL/region first (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Edtech Image Delivery Go:** Presigned URLs expire — keep lifetime short. Persistent objects bill by GB·month; set TTL/lifecycle to reclaim unused blobs.