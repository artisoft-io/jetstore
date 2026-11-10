---
title: "The inference operators, and what a GPU week bought"
description: "Four measured findings about the inference operators that change what you would otherwise do."
kind: explanation
sidebar:
  order: 5
---

**Measured 2026-09-09/10** against the `patient_profile` briefing pipeline over a curated
22-member population. **Full write-up:
[What the inference measurements showed](/docs/developer/compute-pipes/operators/infer-measurements/)**; the shared plumbing
and the `type: infer` backend selection are on [the inference operators page](/docs/developer/compute-pipes/operators/infer/).

Four things here because they are the ones that change what you would otherwise do.

**The serving stack does not affect accuracy.** Across vLLM bf16 eager, vLLM bf16 with CUDA
graphs and ollama `Q4_K_M` — same 22 members, same fact set — every *decidable* accuracy
category was identical; the approximate ones varied inside the run-to-run spread of a
single arm. **So a backend is a cost and latency decision.** Benchmarking one for quality
is a week nobody else needs to spend.

**Throughput has no meaning without a pool size, because the arms cross.** ollama is 1.43x
faster at `pool_size: 1` and vLLM is 1.13x faster at `pool_size: 4` — which is the pool
`patient_profile.pc.json` sets. vLLM's latency rises 10% from pool 1 to 4 and ollama's
rises 79%.

**A slow model load was the disk, not the backend.** vLLM read a 6.34 GiB checkpoint in
51.0 s and ollama ~2.1 GiB in 17.3 s — **124 and ~120 MiB/s, both of them gp3's
unprovisioned baseline**. Neither server is slow at loading. The volume is now provisioned
at 500 MiB/s (`INFER_VOLUME_THROUGHPUT_MIBPS`,
`cdk/jetstore_one/stack/build_infer_ec2.go`), which takes that 51 s to about 13 s. **Check
the volume before concluding a backend is slow to start.**

**A guided-decoding field can be accepted and discarded with a 200 and no warning.** vLLM
v0.28.0 — the version `Dockerfile.infer_service_vllm` pins — takes `guided_json` and
ignores it: 0 of 24 conformant, against 23 of 24 for the same schema sent as
`response_format: {"type":"json_schema"}`. `guided_regex`, `guided_choice` and an invented
field behave the same way, so **this server drops unknown top-level request fields as a
class**. The operator defaults `structured_output` to `json_schema` because of it; leave it
there unless you have measured your server. *The operator reports success on an
unconstrained answer, which is why this cost a day rather than an hour.*
