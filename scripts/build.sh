#!/bin/bash
set -e
VERSION=${1:-1.0.0}
DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

echo "=== TSGMemOptimizer Build v$VERSION ==="
mkdir -p dist

PLATFORMS=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

for p in "${PLATFORMS[@]}"; do
  GOOS=${p%% *}
  GOARCH=${p##* }
  OUT="tsgmemo-v${VERSION}-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && OUT="${OUT}.exe"
  echo "Building $GOOS/$GOARCH -> $OUT"
  GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$VERSION" -o "dist/$OUT" ./cmd/tsgmemo
done

cd dist
sha256sum tsgmemo-v${VERSION}-* > SHA256SUMS.txt
cd ..
echo "=== Done ==="
ls -la dist/
