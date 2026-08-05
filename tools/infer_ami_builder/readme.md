# Building JetStore Infer AMI

Create an Ubuntu-based AMI with NVIDIA drivers for a g5.xlarge instance using HashiCorp Packer,
using the Amazon EBS builder. The build must run on a GPU-enabled instance type such as
g4dn.xlarge or g5.xlarge: the driver itself compiles against kernel headers and would not
strictly need a GPU, but the build's verification step runs `nvidia-smi` on the host and inside a
container before the snapshot is taken, and that requires real hardware. Do not downgrade the
build instance to a non-GPU type — verification will fail.

## Prerequisites and Setup

- **Packer**: ensure Packer is installed on your local machine.
- **AWS credentials**: configure your local terminal with AWS credentials that have permissions to create EC2 instances, security groups, snapshots, and AMIs.

### Install HashiCorp Packer on Linux

Using Official Package Managers, on Ubuntu / Debian:

```bash
# Install dependencies
sudo apt-get update && sudo apt-get install -y gnupg software-properties-common curl

# Add the official HashiCorp GPG key
curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg

# Add the HashiCorp APT repository
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list

# Update system packages and install Packer
sudo apt-get update && sudo apt-get install packer
```
Verify the Installation:

```bash
packer --version
```

## Packer Configuration File

Configuration file name: `infer-nvidia.pkr.hcl`

Build a new AMI from latest Ubuntu linux:

- Install NVIDIA drivers
- Install the native Docker engine
- Install Docker NVIDIA Container Toolkit
- Grant the default ubuntu user non-root permission to execute docker commands without prepending sudo
- Configure Docker to use the NVIDIA runtime as the default container runtime engine
- Cap Docker container logs at 3 files of 50MB each, globally
- Pre-pull the `ollama/ollama:latest` image so it is baked into the AMI
- Reboot, then verify the GPU is visible both on the host and inside a container

The runtime wiring in `/etc/docker/daemon.json` is written by `nvidia-ctk runtime configure`,
and the log rotation policy is merged onto it with `jq`. Do not replace that file wholesale —
the NVIDIA runtime path is version-dependent and is not safe to hardcode.

**Note**: because `default-runtime` is set to `nvidia`, this AMI is GPU-only. Every container
requires the NVIDIA runtime, so `docker run` will fail on a non-GPU instance type.

**Important Hardware Note**: do not switch the cached image to the `ollama/ollama:rocm` tag. The `:rocm` tag is built for AMD GPUs (ROCm runtime), and the target g5.xlarge carries an NVIDIA A10G Tensor Core GPU — running the ROCm build there silently falls back to the CPU.

The configuration pulls the official `ollama/ollama:latest` image. Because NVIDIA is already configured as the default Docker container runtime engine, the regular `ollama/ollama` image detects and locks onto the g5.xlarge NVIDIA GPU without any special tags or `--gpus all` flags.

## Build & Validate Execution

### Configuration

Both settings are optional and read from the environment at build time:

| Variable | Default | Purpose |
|---|---|---|
| `INFER_AMI_REGION` | `us-east-1` | Region to build the AMI in |
| `INFER_AMI_BUILD_INSTANCE_TYPE` | `g4dn.xlarge` | GPU instance used to build the AMI |

An AMI is region-scoped, so `INFER_AMI_REGION` must match the region the JetStore stack deploys
into or the CDK `INFER_AMI_NAME` lookup will not find it.

`INFER_AMI_BUILD_INSTANCE_TYPE` is the throwaway build instance and is **not** the runtime
instance type — that is the CDK's `INFER_EC2_INSTANCE_TYPE` (default `g5.xlarge`). Both must be
GPU-enabled; see the note at the top of this file.

### Build your updated image mapping to AWS infrastructure directly from your CLI terminal:

```bash
packer init infer-nvidia.pkr.hcl
packer build infer-nvidia.pkr.hcl

# Or targeting another region / build instance:
INFER_AMI_REGION=us-west-2 INFER_AMI_BUILD_INSTANCE_TYPE=g5.xlarge packer build infer-nvidia.pkr.hcl
```

The build reboots the temporary instance near the end, then runs `nvidia-smi` on the host and
again inside a container before the snapshot is taken. A driver that fails to load after reboot
fails the build rather than producing a broken AMI, so a successful `packer build` means the GPU
path is already confirmed working.

### Naming

Each build stamps a UTC timestamp into the name so repeated builds don't collide — AMI names must
be unique per account per region:

| | Value |
|---|---|
| AMI name | `jetstore-infer-YYYY-MM-DD-hhmm` |
| `Name` tag (AMI, snapshot, builder instance) | `JetStore Infer YYYY-MM-DD-hhmm` |

The `Name` tag is what the EC2 console shows in its Name column; without it the AMI and its
snapshot appear unnamed.

The CDK stack matches on the **AMI name**, not the tag. `INFER_AMI_NAME` now defaults to
`jetstore-infer-*`, which resolves to the most recently built AMI — so a fresh build is picked up
on the next `cdk deploy` with no configuration change. Set `INFER_AMI_NAME` explicitly only to
pin a specific build.

### Deploy a new g5.xlarge node using the newly generated custom AMI ID output.

Test your instant startup configuration using the cached layers without needing sudo or hardware-pass:

```bash
# Will start immediately since the layers are baked into the AMI storage drive natively.
# The named volume keeps pulled model weights outside the container's writable layer,
# so they survive `docker rm` and a container upgrade.
docker run -d -p 11434:11434 -v ollama:/root/.ollama --name ollama-gpu ollama/ollama:latest
```

Verify your container is leveraging the NVIDIA A10G hardware on your G5 host:

```bash
docker exec -it ollama-gpu nvidia-smi
```

## (Draft) What's Next To Do:

- Preload specific LLM weights (like llama3 or mistral) inside the AMI so Ollama doesn't have to download them over the internet at startup

- Add AWS User Data launch script to automatically spin up the Ollama container the moment the EC2 instance powers on
