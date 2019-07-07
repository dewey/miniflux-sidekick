source develop.env

function cleanup() {
    rm -f miniflux-sidekick
}
trap cleanup EXIT

# Compile Go
GO111MODULE=on GOGC=off go build -mod=vendor -v -o miniflux-sidekick ./cmd/api/
./miniflux-sidekick
