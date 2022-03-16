FROM golang:1.18 as builder

ENV CGO_ENABLED=0

WORKDIR /usr/app/src

COPY go.mod go.sum ./
RUN go mod tidy

COPY . .
RUN go build

FROM alpine

RUN addgroup -S autoscaler && \
  adduser -S -g autoscaler -u 20014 autoscaler

USER autoscaler

COPY --from=builder /usr/app/src/autoscaler /bin/autoscaler

WORKDIR /bin/

ENTRYPOINT [ "/bin/autoscaler" ]
