<h1 align="center">go-k8s-process</h1>

<p align="center">
    <a href="https://github.com/k8-proxy/go-k8s-process/actions/workflows/build.yml">
        <img src="https://github.com/k8-proxy/go-k8s-process/actions/workflows/build.yml/badge.svg"/>
    </a>
    <a href="https://codecov.io/gh/k8-proxy/go-k8s-process">
        <img src="https://codecov.io/gh/k8-proxy/go-k8s-process/branch/main/graph/badge.svg"/>
    </a>	    
    <a href="https://goreportcard.com/report/github.com/k8-proxy/go-k8s-process">
      <img src="https://goreportcard.com/badge/k8-proxy/go-k8s-process" alt="Go Report Card">
    </a>
	<a href="https://github.com/k8-proxy/go-k8s-process/pulls">
        <img src="https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat" alt="Contributions welcome">
    </a>
    <a href="https://opensource.org/licenses/Apache-2.0">
        <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="Apache License, Version 2.0">
    </a>
    <a href="https://github.com/k8-proxy/go-k8s-process/releases/latest">
        <img src="https://img.shields.io/github/release/k8-proxy/go-k8s-process.svg?style=flat"/>
    </a>
</p>

# go-k8s-process

This is the service that will process/rebuild the file, as per the projet defined in [this repo](https://github.com/k8-proxy/go-k8s-infra)
These pods are controlled the rebuild pods controller.

### Steps of processing
- Listens on the processing request queue to get files pending for a rebuild
- Get the file on queue
- Copy it to local folder
- Run the rebuild service
- Get the rebuild result
- Upload the result file to minio
- Notifies the queue
- Exit


### Docker build
- To build the docker image
```
git clone https://github.com/k8-proxy/go-k8s-process.git
cd go-k8s-process
docker build -t <docker_image_name> .
```

- To run the container
First make sure that you have rabbitmq and minio running, then run the command bellow 

```
docker run -e ADAPTATION_REQUEST_QUEUE_HOSTNAME='<rabbit-host>' \ 
-e ADAPTATION_REQUEST_QUEUE_PORT='<rabbit-port>' \
-e MESSAGE_BROKER_USER='<rabbit-user>' \
-e MESSAGE_BROKER_PASSWORD='<rabbit-password>' \
-e MINIO_ENDPOINT='<minio-endpoint>' \ 
-e MINIO_ACCESS_KEY='<minio-access>' \ 
-e MINIO_SECRET_KEY='<minio-secret>' \ 
-e MINIO_SOURCE_BUCKET='<bucket-to-upload-file>' \ 
--name <docker_container_name> <docker_image_name>
```

# Testing steps
- Run srv1 as per https://github.com/k8-proxy/go-k8s-srv1
- Run the container as mentionned above

- Publish data reference to rabbitMq on queue name : adaptation-request-queue with the following data(table) :
* file-id : An ID for the file
* source-file-location : The full path to the file
* rebuilt-file-location : A full path representing the location where the rebuilt file will go to


- Check your container logs to see the processing

```
docker logs <container name>
```

# Rebuild flow to implement

![new-rebuild-flow-v2](https://github.com/k8-proxy/go-k8s-infra/raw/main/diagram/go-k8s-infra.png)

