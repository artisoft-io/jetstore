packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.8"
      source  = "://github.com"
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
      "sudo apt-get install -y build-essential gcc make ca-certificates curl gnupg linux-headers-$(uname -r)",

      "echo '=== 2. Installing NVIDIA Host Drivers ==='",
      "sudo apt-get install -y nvidia-driver-535-server",

      "echo '=== 3. Purging Snap Docker ==='",
      "sudo snap remove --purge docker || true",
      "sudo apt-get remove -y docker docker-engine docker.io containerd runc || true",

      "echo '=== 4. Installing Official Docker CE Repository & Engine ==='",
      "sudo install -m 0755 -d /etc/apt/keyrings",
      "sudo curl -fsSL https://docker.com -o /etc/apt/keyrings/docker.asc",
      "sudo chmod a+r /etc/apt/keyrings/docker.asc",
      "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://docker.com $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
      "sudo apt-get update -y",
      "sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",

      "echo '=== 5. Adding NVIDIA Container Toolkit Repository ==='",
      "curl -fsSL https://github.io | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg",
      "curl -s -L https://github.io | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list",
      "sudo apt-get update -y",
      "sudo apt-get install -y nvidia-container-toolkit",

      "echo '=== 6. Configuring NVIDIA Container Toolkit & Global Logging Rotation ==='",
      "# Configures Docker to automatically use NVIDIA GPU passthrough as the default engine layout",
      "sudo nvidia-ctk runtime configure --runtime=docker --set-as-default",
      
      "# Inject global json policies to restrict log files to 3 files maximum at 50 megabytes each",
      "sudo tee /etc/docker/daemon.json <<EOF",
      "{",
      "    \"default-runtime\": \"nvidia\",",
      "    \"runtimes\": {",
      "        \"nvidia\": {",
      "            \"path\": \"nvidia-container-runtime\",",
      "            \"runtimeArgs\": []",
      "        }",
      "    },",
      "    \"log-driver\": \"json-file\",",
      "    \"log-opts\": {",
      "        \"max-size\": \"50m\",",
      "        \"max-file\": \"3\"",
      "    }",
      "}",
      "EOF",
      "sudo systemctl restart docker",

      "echo '=== 7. Granting Non-Root Docker Access to Ubuntu User ==='",
      "sudo usermod -aG docker ubuntu",

      "echo '=== 8. Pre-downloading (Caching) Docker Image Layers ==='",
      "# Pulling the official GPU-compatible image directly into the AMI storage layout",
      "sudo docker pull ollama/ollama:latest",

      "echo '=== 9. Cleaning up Image Temporary Files ==='",
      "sudo apt-get clean",
      "sudo rm -rf /var/lib/apt/lists/*",
      "history -c"
    ]
  }
}
