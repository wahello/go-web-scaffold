FROM golang:1-alpine3.14 as golang
RUN apk --no-cache add make git zip tzdata ca-certificates nodejs npm gcc musl-dev
WORKDIR /app
COPY . .
RUN make

FROM alpine:3.14
# Dependencies
RUN apk --no-cache add tzdata ca-certificates musl
# where application lives
WORKDIR /app
# Copy the products
COPY --from=golang /app/bin .
# env
ENV GIN_MODE="release"
EXPOSE 3000
ENTRYPOINT ["/app/telescope"]