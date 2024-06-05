
FROM registry.ci.openshift.org/openshift/release:rhel-8-release-golang-1.22-openshift-4.17 AS gobuilder

ENV BASE=github.com/openshift-service-mesh/sail-operator
WORKDIR ${GOPATH}/src/${BASE}

COPY . .

# TODO: is this needed?
# ENV GOFLAGS="-mod=vendor"
ENV BUILD_WITH_CONTAINER=0

RUN make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest 
# gcr.io/distroless/static:nonroot

ARG TARGETOS TARGETARCH
RUN env

COPY --from=gobuilder /go/src/github.com/openshift-service-mesh/sail-operator/out/${TARGETOS:-linux}_${TARGETARCH:-amd64}/manager /manager
COPY --from=gobuilder /go/src/github.com/openshift-service-mesh/sail-operator/resources /var/lib/sail-operator/resources

USER 65532:65532
WORKDIR /
ENTRYPOINT ["/manager"]
