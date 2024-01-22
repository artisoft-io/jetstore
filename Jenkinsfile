node {
    // Build the images locally
    sh 'ls -la'
    docker.build("jetstore_base_builder:go-bullseye", "-f Dockerfile.go_bullseye_base .")    
    docker.build("jetstore_builder:go-bullseye", "-f Dockerfile.go_bullseye .")    
    docker.build("jetstore_base:bullseye", "-f Dockerfile.bullseye_base .")    
    docker.build("jetstore:bullseye", "-f Dockerfile.rt_bullseye .")    
}
