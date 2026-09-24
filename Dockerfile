FROM golang:1.27.1 as builder

ENV CGO_ENABLED=0

WORKDIR /usr/app/src

COPY go.mod go.sum ./
RUN go mod tidy

ARG VERSION=development

COPY . .
RUN go build -ldflags "-X main.version=${VERSION}"

FROM alpine

RUN addgroup -S autoscaler && \
  adduser -S -g autoscaler -u 20014 autoscaler

USER autoscaler

COPY --from=builder /usr/app/src/autoscaler /bin/autoscaler

WORKDIR /bin/

ENTRYPOINT [ "/bin/autoscaler" ]
