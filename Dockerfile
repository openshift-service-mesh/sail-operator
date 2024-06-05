
FROM registry.ci.openshift.org/openshift/release:rhel-8-release-golang-1.21-openshift-4.16 AS gobuilder

COPY . .

ENV GOFLAGS="-mod=vendor"
ENV BUILD_WITH_CONTAINER=0

RUN make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest 
# gcr.io/distroless/static:nonroot

ARG TARGETOS TARGETARCH

COPY --from=gobuilder out/${TARGETOS:-linux}_${TARGETARCH:-amd64}/manager /manager
COPY --from=gobuilder resources /var/lib/sail-operator/resources

USER 65532:65532
WORKDIR /
ENTRYPOINT ["/manager"]
