# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-azure
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager sigs.k8s.io/cluster-api-provider-azure/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:3.8
WORKDIR /root/
COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-azure/manager .
COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resourcemanagement/template/deployment-template.json .
ENTRYPOINT ["./manager"]
