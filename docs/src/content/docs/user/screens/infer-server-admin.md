---
title: Infer Server Admin
description: The Infer Server Admin screen, which starts and stops the installation's GPU inference server and manages the models it serves.
kind: reference
---

The *Infer Server Admin* screen starts and stops the installation's **inference server**, the GPU
service that runs the language models a pipeline's inference steps call, and manages the models on
it. The server is optional and is normally stopped, because it runs on paid GPU capacity; an
installation built without it has nothing for this screen to act on. How it is deployed and what it
costs is in [Start and stop the inference server](/docs/install/how-to/start-and-stop-the-infer-server/).

**Reach it** from *Infer Server Admin* in the top bar. Every button needs the `infer_server_admin`
capability; without it the entry is disabled, and the server refuses every request.

This page has no screenshot: the screen reads the server's state from AWS, which the documentation's
screenshots are not taken against.

## The controls

The four model buttons only fill *Request*; nothing is sent until **Submit**.

| Control | What it does |
|---|---|
| **Start** | Starts the inference service and the GPU instance behind it. Enabled unless the server is running. It takes several minutes; the status says *starting* until it is up. |
| **Stop** | Asks *Stop the Infer Server?* first: the GPU instance is terminated and any loaded model is unloaded. **Stop it** confirms. Enabled unless the server is stopped. |
| **Models** | Fills *Request* with a request that lists the models currently loaded in memory. |
| **Pull Model** | Fills *Request* with a request that downloads the default model onto the server. Change the model's name to pull another. |
| **Model Details** | Fills *Request* with a request for the default model's details. |
| **Delete** | Fills *Request* with a request that deletes the default model from the server. |
| **Infer Server** | The status, read when the screen opens: *running*, *stopped*, *starting* or *stopping*, with the running and desired tasks and instances. |
| **Refresh** | Reads the status again. |
| **Request** | The request to send, as JSON. A button above fills it; you can also paste or edit one. |
| **Response** | The server's answer to the last request. |
| **Submit** | Sends *Request* and shows the answer in *Response*. Enabled when *Request* holds something. |

A request names an **action** and a **body**, which is passed to the model server as it is:

```json
{ "action": "pull_model", "body": { "model": "granite4.1:3b", "stream": false } }
```

The actions are `list_models`, `pull_model`, `show_model` and `delete_model`, and `server_status`,
`start_server` and `stop_server`, which the buttons send. The server accepts these and no other: a
request cannot name an address of its own.

**Starting and stopping are idempotent.** Asking to start a server that is already running, or to stop
one already stopped, changes nothing and says so.
