FROM golang:1.22
LABEL authors="julfikar"

WORKDIR /app
COPY . .

RUN go build -o jerusalem-tunnel -v ./...
ENTRYPOINT ["./jerusalem-tunnel"]

EXPOSE 1024-1100

# Set the image and container name
LABEL name="jerusalem-tunnel-container"
LABEL image="jerusalem-tunnel-image"