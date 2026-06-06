# Image for the unified `sakumock` binary. GoReleaser builds the cross-platform
# binaries and feeds the matching one into this build context per platform, so
# the Dockerfile only needs to copy it (no toolchain, no compilation here).
FROM gcr.io/distroless/static-debian12:nonroot

COPY sakumock /sakumock

# One port per service (see the service table in README); EXPOSE is documentation
# only — publish the ports you need with `-p`.
EXPOSE 18080 18081 18082 18083 18084

ENTRYPOINT ["/sakumock"]
# Run every service bound to 0.0.0.0 so published ports are reachable from
# outside the container. Override (e.g. `docker run ... env --host localhost`)
# to emit client env vars instead.
CMD ["all", "--listen-host", "0.0.0.0"]
