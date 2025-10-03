# Multi-stage build for Go Lambda with C++ library
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    g++ \
    musl-dev \
    make \
    pkgconfig

# Set working directory
WORKDIR /app

# Copy C++ source files
COPY cpp/ ./cpp/

# Build the C++ library
WORKDIR /app/cpp
RUN make clean && make

# Go back to app directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download Go dependencies
RUN go mod download

# Copy Go source code
COPY main.go .

# Set CGO environment variables
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# Build the Go binary with CGO enabled
RUN go build -ldflags="-w -s" -o main main.go

# Final stage - use AWS Lambda base image (Amazon Linux 2)
FROM public.ecr.aws/lambda/provided:al2

# Install runtime dependencies
RUN yum update -y && \
    yum install -y libstdc++ && \
    yum clean all

# Copy the binary and C++ library from builder stage
COPY --from=builder /app/main ${LAMBDA_TASK_ROOT}
COPY --from=builder /app/cpp/libhello.so /usr/local/lib/

# Set library path environment variable instead of using ldconfig
ENV LD_LIBRARY_PATH=/usr/local/lib:/usr/lib64:/lib64:$LD_LIBRARY_PATH

# Create a simple library cache manually
RUN echo "/usr/local/lib" > /etc/ld.so.conf.d/local.conf

# Set the CMD to your handler
CMD [ "main" ]
