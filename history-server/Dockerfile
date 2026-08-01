# Massyl Benarab (ORCID: 0009-0006-9405-3374)
# Younes Aoures (ORCID: 0009-0009-6551-1294)

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY shared/ ./shared/
COPY history-server/ ./history-server/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /history-server ./history-server
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /history-server /usr/local/bin/history-server
EXPOSE 8080
ENTRYPOINT ["history-server"]
