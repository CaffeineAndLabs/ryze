FROM golang:1.11.3-alpine AS builder

RUN apk add --update --no-cache git ca-certificates zip

WORKDIR /src
COPY . ./

RUN GIT_COMMIT=$(git rev-list -1 HEAD) && \
    CGO_ENABLED=0 go build \
    -mod=vendor \
    -installsuffix 'static' \
    -ldflags "-X main.GitCommit=$GIT_COMMIT" \
    -o /ryze .

FROM scratch AS final

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /ryze /ryze

EXPOSE 8000

ENTRYPOINT ["/ryze"]
