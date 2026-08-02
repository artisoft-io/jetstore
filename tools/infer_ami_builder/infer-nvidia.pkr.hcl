packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.8"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

source "amazon-ebs" "ubuntu-nvidia-docker" {
  ami_name      = "ubuntu-2404-nvidia-ollama-{{timestamp}}"
  instance_type = "g4dn.xlarge" # Worker instance with an attached GPU to compile drivers
  region        = "us-east-1"

  source_ami_filter {
    filters = {
      name                = "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }

  ssh_username = "ubuntu"

  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = 50 # Expanded to handle heavy container layer caching
    volume_type           = "gp3"
    delete_on_termination = true
  }
}

build {
  name    = "nvidia-docker-complete-builder"
  sources = ["source.amazon-ebs.ubuntu-nvidia-docker"]

  provisioner "shell" {
    inline = [
      "set -e",

      "echo '=== 1. Updating System & Installing Kernel Headers ==='",
      "sudo apt-get update -y",
      "sudo apt-get install -y build-essential gcc make ca-certificates curl gnupg jq linux-headers-$(uname -r)",

      "echo '=== 2. Installing NVIDIA Host Drivers ==='",
      "sudo apt-get install -y nvidia-driver-535-server",

      "echo '=== 3. Purging Snap Docker ==='",
      "sudo snap remove --purge docker || true",
      "sudo apt-get remove -y docker docker-engine docker.io containerd runc || true",

      "echo '=== 4. Installing Official Docker CE Repository & Engine ==='",
      "sudo install -m 0755 -d /etc/apt/keyrings",
      "sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc",
      "sudo chmod a+r /etc/apt/keyrings/docker.asc",
      "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
      "sudo apt-get update -y",
      "sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",

      "echo '=== 5. Adding NVIDIA Container Toolkit Repository ==='",
      "curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg",
      "curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list",
      "sudo apt-get update -y",
      "sudo apt-get install -y nvidia-container-toolkit",

      "echo '=== 6. Configuring NVIDIA Container Toolkit & Global Logging Rotation ==='",
      "# nvidia-ctk owns the runtime wiring in daemon.json; the runtime path can change between toolkit versions",
      "sudo nvidia-ctk runtime configure --runtime=docker --set-as-default",

      "# Merge in log rotation (3 files max, 50MB each) without clobbering the runtime config written above",
      "sudo jq '. + {\"log-driver\": \"json-file\", \"log-opts\": {\"max-size\": \"50m\", \"max-file\": \"3\"}}' /etc/docker/daemon.json > /tmp/daemon.json",
      "sudo install -m 0644 -o root -g root /tmp/daemon.json /etc/docker/daemon.json",
      "rm -f /tmp/daemon.json",
      "sudo systemctl restart docker",

      "echo '=== 7. Granting Non-Root Docker Access to Ubuntu User ==='",
      "sudo usermod -aG docker ubuntu",

      "echo '=== 8. Pre-downloading (Caching) Docker Image Layers ==='",
      "# Pulling the official GPU-compatible image directly into the AMI storage layout",
      "sudo docker pull ollama/ollama:latest",

      "echo '=== 9. Cleaning up Image Temporary Files ==='",
      "sudo apt-get clean",
      "sudo rm -rf /var/lib/apt/lists/*"
    ]
  }

  # Reboot before verifying: this proves the DKMS module survives a fresh boot,
  # which is the exact path every instance launched from this AMI will take.
  provisioner "shell" {
    expect_disconnect = true
    inline = [
      "echo '=== 10. Rebooting to load the NVIDIA kernel module ==='",
      "sudo reboot"
    ]
  }

  provisioner "shell" {
    pause_before = "30s"
    inline = [
      "echo '=== 11. Verifying the GPU is visible on the host ==='",
      "nvidia-smi",

      "echo '=== 12. Verifying NVIDIA is the default container runtime ==='",
      "# No --gpus flag: this fails unless default-runtime is wired up correctly",
      "sudo docker run --rm --entrypoint nvidia-smi ollama/ollama:latest"
    ]
  }
}
