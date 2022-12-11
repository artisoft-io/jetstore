FROM golang:1.19
WORKDIR /src/
COPY . /src/
RUN go build -o app

FROM ubuntu:latest
RUN apt-get update && apt-get install -y ca-certificates
WORKDIR /run/
COPY --from=0 /src/app ./
CMD [ "./app" ]
