FROM golang:1.24.2-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o /relaydesk ./cmd/relaydesk
FROM alpine:3.21
RUN addgroup -g 10001 relaydesk && adduser -D -u 10001 -G relaydesk relaydesk && mkdir /data && chown relaydesk:relaydesk /data
COPY --from=build /relaydesk /usr/local/bin/relaydesk
USER 10001:10001
EXPOSE 8787
VOLUME /data
ENTRYPOINT ["relaydesk"]
CMD ["gateway", "--listen", "0.0.0.0:8787", "--state", "/data/state.json"]
