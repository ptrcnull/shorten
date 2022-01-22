FROM golang:latest as builder

LABEL maintainer="ptrcnull <docker@ptrcnull.me>"

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server main.go

FROM scratch

WORKDIR /app
COPY --from=builder /src/server .

CMD ["/app/server"]
