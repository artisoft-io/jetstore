---
title: "Sending to S3: what is retried, and what cannot be"
description: "How compute pipes and the reports send objects to S3 under throttling: a bounded, jittered retry on throttling and transient errors, a body re-read from its start on each attempt, a streamed body sent once, and one S3 client per process."
kind: explanation
sidebar:
  order: 17
---

**Recorded 2026-11-27.**

**The symptom.** Several pipelines started at once, each with many nodes writing partitions, failed on
S3 sends and on file handles instead of waiting. Starting the same runs again about twenty minutes
later worked.

## What a send does now

Every send of an object body or an S3-to-S3 copy goes through `awsi.RetrySend`
(`jets/awsi/s3_send_retry.go`). This covers the partition writers' S3 worker (`S3DeviceWorker`), the
`awsi.UploadToS3`, `UploadToS3FromReader` and `UploadBufToS3` uploads, and the single-part copy in
`awsi.MultiPartCopy`. A failed send is tried again only when the error is one that may change:

- S3 throttled it (`SlowDown` and the other throttling codes the SDK knows);
- the SDK classes it as transient (a 500, 502, 503 or 504, a request timeout, a connection reset);
- the SDK's own retry quota ran out (`retry quota exceeded`).

An error that will not change is returned at once, such as access denied, a missing bucket, or a
cancelled context. Before each retry the send waits a jittered, doubling backoff of about 1, 2, 4, 8,
16, 30 and 30 seconds. Each wait is drawn from the upper half of its step. The send gives up after
eight attempts, roughly 45 to 90 seconds in all. Each retry is logged with the object, the attempt
and the error:

```
S3 send s3://<bucket>/<key>: attempt 2 of 8 failed, retrying in 1.852s: ... api error SlowDown: ...
```

The SDK still retries each request on its own underneath this, as it always has.

## A body is re-read from its start, and a stream is sent once

**Until 2026-11-27 a retry sent whatever the failed attempt had left unread.** The S3 worker retried
six times at 500 ms on any error, and so did `UploadToS3FromReader`. Both did it with the body the
failed attempt had already read from. For a partition file smaller than a part, the first attempt
reads the whole file, so the retry sent an **empty object and succeeded**. This was measured with a
fake S3 that answers the first PUT with 503: the old worker sent the file, then `""`. A larger body
lost its first parts instead. The run completed with a partition missing or truncated in S3, and
nothing in the log said so.

Now the worker seeks its local file back to the start before each attempt, and the upload functions
seek a body that can seek (a file, a `bytes.Reader`, a `strings.Reader`) back to where it started.
**A body that cannot seek is sent once.** That is a pipe: a partition written with
`stream_data_out`, or a merge that streams its output. For these, only the SDK's own per-part
retries apply. Retrying one would mean holding the whole object in memory or on disk, which is what
streaming avoids. A streamed partition that fails now fails the run. Before, `WritePartition` dropped
the upload's error, the partition was missing from a run that succeeded, and the goroutine writing
into the pipe stayed blocked for the life of the process.

## One S3 client per process

**Until 2026-11-27 `awsi.NewS3Client` built a new client, and so a new HTTP transport, on every
call.** Every upload and download in `awsi` calls it. After its last request, a transport keeps up to
ten idle connections per host open for 90 seconds (the SDK's `IdleConnTimeout`). A transport that is
no longer referenced is not closed when it is collected. So each call left its sockets, which are
file descriptors, open for a minute and a half after it returned. A node that streams many
partitions, at ten connections each, could reach the process's descriptor limit. This is reasoned
from the SDK's transport defaults and was not measured on a cluster. It is the likeliest explanation
for the file-handle failures in the symptom above.

`NewS3Client` now returns one client per process, created on first use. An `s3.Client` is safe for
concurrent use and refreshes its own credentials. One shared client would also share the SDK's retry
quota, which a burst of throttling empties faster. So the shared client's retryer has no quota
(`ratelimit.None`): the SDK retries each request with its usual backoff and attempt limit, and
`RetrySend` bounds the rest. The partition writers' S3 worker pool still has its own client, made
once per node (`NewS3DeviceManager`).
