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

### Dev section

the k8-process add additional values to the rabbitmq header wich contain additional inforamtions :
```
- "rebuild-sdk-version" which is the sdk version
```
```
- "rebuild-processing-status" which is the status of the rebuild processing operations , there is five states values :

 "UNPROCESSABLE" if the file can't be processed for example not supported file type
 "CLEAN" if the original file is clean so it remain unchanged
 "CLEANED" if the file is cleaned by the rebuil engine 
 "INTERNALL ERROR" if there is error in the k8-process or the rebuilt ClI while procesing the file, 
 "SDK EXPIRED" if the rebuilt SDK expired
```
```
- "clean-presigned-url" which is the rebuilt file presigned url
```
```
- "report-presigned-url" which is the report file presigned url
```
```
- "gwlog-presigned-url" which is the glassWall log file presgined url
```
```
- "log-pesined-url" which is the log file presgined url
```

### Steps of processing
- Listens on the processing request queue to get files pending for a rebuild
- Get the file on queue
- Copy it to local folder
- Run the rebuild service
- Get the rebuild result
- Upload the result file to minio
- Notifies the queue
- Exit

### Sdk-rebuild-eval version
### 1.221

### Docker build for local testing
- To build for local testing 
```
git clone https://github.com/k8-proxy/go-k8s-process.git
cd go-k8s-process
git submodule update --init
go build -o  go-k8s-process ./cmd
export ADAPTATION_REQUEST_QUEUE_HOSTNAME='<rabbit-host>' \ 
ADAPTATION_REQUEST_QUEUE_PORT='<rabbit-port>' \
MESSAGE_BROKER_USER='<rabbit-user>' \
MESSAGE_BROKER_PASSWORD='<rabbit-password>' \
MINIO_ENDPOINT='<minio-endpoint>' \ 
MINIO_ACCESS_KEY='<minio-access>' \ 
MINIO_SECRET_KEY='<minio-secret>' \ 
MINIO_SOURCE_BUCKET='<bucket-to-upload-file>' \ 
GWCLI='<cli-path>'  \
INICONFIG='<ini-config-path>' \
XMLCONFIG='<xml-config-path>'


```
### Installing the Glasswall Rebuild SDK

Copy the `libglasswall.classic.so` shared library into the `/usr/lib` folder.
```
cp ./sdk-rebuild-eval/libs/rebuild/linux/libglasswall.classic.so /usr/lib
```


### Inform System about the Glasswall Rebuild SDK 
Once in place the library needs to be registered to make it accessible. Create a `glasswall.classic.conf` file, with the installed location
```
echo "/usr/lib" > glasswall.classic.conf
```
Update the `etc` directory
```
sudo cp glasswall.classic.conf /etc/ld.so.conf.d
```
Run `ldconfig` to configure the dynamic linker run-time bindings
```
sudo ldconfig
```

Check that the Glasswall library has been installed
```
sudo ldconfig -p | grep glasswall.classic
```
Remove the `.conf` file
```
rm glasswall.classic.conf
```


### Docker build
- To build the docker image
```
git clone https://github.com/k8-proxy/go-k8s-process.git
cd go-k8s-process
git submodule update --init
docker build -t <docker_image_name> .
```

- To run the container
First make sure that you have rabbitmq  minio and jaeger running, then run the command bellow 
note to activate Jaeger trace set JAEGER_AGENT_ON=true
```
docker run -e ADAPTATION_REQUEST_QUEUE_HOSTNAME='<rabbit-host>' \ 
-e ADAPTATION_REQUEST_QUEUE_PORT='<rabbit-port>' \
-e MESSAGE_BROKER_USER='<rabbit-user>' \
-e MESSAGE_BROKER_PASSWORD='<rabbit-password>' \
-e MINIO_ENDPOINT='<minio-endpoint>' \ 
-e MINIO_ACCESS_KEY='<minio-access>' \ 
-e MINIO_SECRET_KEY='<minio-secret>' \ 
-e MINIO_SOURCE_BUCKET='<bucket-to-upload-file>' \ 
-e JAEGER_AGENT_HOST='<jaeger-host>' \
-e JAEGER_AGENT_PORT='<jaeger-port>' \
-e JAEGER_AGENT_ON=true \
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
# known issues

the rebuild CLI have some problems working on alpine , maybe there is missing dependencies or directory structrure , so instead we use ubuntu base image  for the moment

# Rebuild flow to implement

![new-rebuild-flow-v2](https://github.com/k8-proxy/go-k8s-infra/raw/main/diagram/go-k8s-infra.png)

