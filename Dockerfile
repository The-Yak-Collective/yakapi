FROM golang:1.19

WORKDIR /usr/src/yakapi

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 go build -v -o /yakapi .

# TODO: scratch is probably too restrictive
FROM scratch
COPY --from=0 /yakapi /yakapi

ENV YAKAPI_PORT=8080
ENV YAKAPI_NAME="Yak Bot"
ENV YAKAPI_PROJECT_URL="https://github.com/The-Yak-Collective/yakrover"
ENV YAKAPI_CI_ADAPTER="cat"

CMD ["/yakapi"]