FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags "-s -w -X 'github.com/m-mdy-m/gix/internal/commands.Version=${VERSION}'" \
      -o /out/gix ./cmd/gix

FROM alpine:3.20

RUN apk add --no-cache git ca-certificates \
 && adduser -D -u 10001 gix \
 && git config --system --add safe.directory '*'

COPY --from=builder /out/gix /usr/local/bin/gix

ENV HOME=/home/gix

USER gix
WORKDIR /work

ENTRYPOINT ["gix"]
CMD ["--help"]
