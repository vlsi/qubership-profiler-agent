# Build context: repository root. Run make -C apps/profiler-backend docker-binaries first.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

ARG TARGETOS
ARG TARGETARCH

COPY --chmod=755 apps/profiler-backend/build/image/${TARGETOS}_${TARGETARCH}/profiler-backend /usr/local/bin/profiler-backend
# Named volumes inherit this directory's ownership on first mount.
COPY --chown=65532:65532 apps/profiler-backend/build/image/data/ /data/

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/profiler-backend"]
