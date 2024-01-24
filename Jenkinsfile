node {
    // Build the images locally
    docker.build("jetstore_base_builder:go-bullseye", "-f Dockerfile.go_bullseye_base .")    
    docker.build("jetstore_builder:go-bullseye", "-f Dockerfile.go_bullseye .")    
    docker.build("jetstore_base:bullseye", "-f Dockerfile.bullseye_base .")    
    docker.build("jetstore:bullseye", "-f Dockerfile.rt_bullseye .")    
}
node {
    // Get jetstore repo from repository
    sh 'rm -rf jetstore'
    sh "git clone 'https://github.com/artisoft-io/jetstore.git'"

    // Build the jetstore images locally
    docker.build("jetstore_base_builder:go-bullseye", "-f jetstore/Dockerfile.go_bullseye_base ./jetstore")    
    docker.build("jetstore_builder:go-bullseye", "-f jetstore/Dockerfile.go_bullseye ./jetstore")    
    docker.build("jetstore_base:bullseye", "-f jetstore/Dockerfile.bullseye_base ./jetstore")    
    docker.build("jetstore:bullseye", "-f jetstore/Dockerfile.rt_bullseye ./jetstore")

}

// was working
node {
    // Get some code from a GitHub repository
    //sh 'rm -rf jetstore'
    //sh "git clone 'https://github.com/artisoft-io/jetstore.git'"

    // Build the images locally
    docker.build("jetstore_base_builder:go-bullseye", "-f jetstore/Dockerfile.go_bullseye_base ./jetstore")    
}


node {
    // Get some code from a GitHub repository
    git branch: 'main', url: 'https://github.com/artisoft-io/jetstore.git'

    // Build the images locally
    docker.build("jetstore_base_builder:go-bullseye", "-f Dockerfile.go_bullseye_base .")
    docker.build("jetstore_builder:go-bullseye", "-f Dockerfile.go_bullseye .")
    docker.build("jetstore_base:bullseye", "-f Dockerfile.bullseye_base .")
    docker.build("jetstore:bullseye", "-f Dockerfile.rt_bullseye .")
}

