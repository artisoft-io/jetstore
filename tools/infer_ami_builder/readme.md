# Building JetStore Infer AMI

Create an Ubuntu-based AMI with NVIDIA drivers for a g5.xlarge instance using HashiCorp Packer, 
using the Amazon EBS builder. Because NVIDIA drivers require a GPU to compile and install correctly during the build phase, Packer must run the temporary build instance on a GPU-enabled instance type like g4dn.xlarge or g5.xlarge.

## Prerequisites and SetupInstall Packer: 

- Ensure Packer is installed on your local machine.AWS 
- Credentials: Configure your local terminal with AWS credentials that have permissions to create EC2 instances, security groups, snapshots, and AMIs.

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

**Important Hardware Note**: Caching the ollama/ollama:rocm image. The :rocm tag is specifically built for AMD GPUs (ROCm runtime). Because your target instance type is an AWS g5.xlarge, it features an NVIDIA A10G Tensor Core GPU. Running the ROCm version on an NVIDIA instance will force it to fall back to the CPU.

The configuration below pulls the official ollama/ollama:latest image instead. Because we already configured NVIDIA as the default Docker container runtime engine, the regular ollama/ollama image will automatically detect and lock onto your g5.xlarge NVIDIA GPU without needing any special tags or --gpus all flags!

## Build & Validate Execution

### Build your updated image mapping to AWS infrastructure directly from your CLI terminal:

```bash
packer init ubuntu-nvidia-docker-complete.pkr.hcl
packer build ubuntu-nvidia-docker-complete.pkr.hcl
```
### Deploy a new g5.xlarge node using the newly generated custom AMI ID output.

Test your instant startup configuration using the cached layers without needing sudo or hardware-pass:

```bash
# Will start immediately since the layers are baked into the AMI storage drive natively
docker run -d -p 11434:11434 --name ollama-gpu ollama/ollama:latest
```

Verify your container is leveraging the NVIDIA A10G hardware on your G5 host:

```bash
docker exec -it ollama-gpu nvidia-smi
```

## (Draft) What's Next To Do:

- Preload specific LLM weights (like llama3 or mistral) inside the AMI so Ollama doesn't have to download them over the internet at startup

- Add AWS User Data launch script to automatically spin up the Ollama container the moment the EC2 instance powers on
