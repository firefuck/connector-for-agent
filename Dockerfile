# Official multi-platform image indexes, reviewed 2026-11-06. Update tags and
# digests together after running packaging/test-container.py.
FROM golang:1.26.10-alpine3.24@sha256:87c65a688975c0b1d4585d204874c45ca78354670e3a5f11368c5a492303d696 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o /relaydesk ./cmd/relaydesk
FROM alpine:3.24.4@sha256:3f4d6b32e26703c6082839234f43769d5046b3ac7c910033911d1e71d8cd630a
RUN addgroup -g 10001 relaydesk && adduser -D -u 10001 -G relaydesk relaydesk && mkdir /data && chown relaydesk:relaydesk /data
COPY --from=build /relaydesk /usr/local/bin/relaydesk
USER 10001:10001
EXPOSE 8787
VOLUME /data
# BusyBox wget does not consistently honor no_proxy. This request stays inside
# the container's loopback interface, including on hosts with proxy defaults.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD wget -Y off -q -O /dev/null http://127.0.0.1:8787/health || exit 1
ENTRYPOINT ["relaydesk"]
CMD ["gateway", "--listen", "0.0.0.0:8787", "--state", "/data/state.json"]
