#!/usr/bin/env sh
set -eu

REPO="${GAME_DEPLOY_REPO:-neko233-com/game-deploy-cli}"
VERSION="${GAME_DEPLOY_VERSION:-latest}"
INSTALL_DIR="${GAME_DEPLOY_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Linux) OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) echo "unsupported OS; use install.ps1 on Windows" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/$REPO/releases/latest/download"
else
  VERSION="${VERSION#v}"
  BASE_URL="https://github.com/$REPO/releases/download/v$VERSION"
fi

ASSET="game-deploy-$OS-$ARCH.tar.gz"
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t game-deploy)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$BASE_URL/$ASSET" -o "$TMP_DIR/$ASSET"
else
  wget -qO "$TMP_DIR/$ASSET" "$BASE_URL/$ASSET"
fi
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP_DIR/game-deploy" "$INSTALL_DIR/game-deploy"

case ":${PATH:-}:" in
  *":$INSTALL_DIR:"*) : ;;
  *)
    PROFILE="${GAME_DEPLOY_PROFILE:-$HOME/.profile}"
    if [ ! -f "$PROFILE" ] || ! grep -Fq '# game-deploy' "$PROFILE"; then
      printf '\n# game-deploy\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$PROFILE"
    fi
    ;;
esac

echo "game-deploy installed to $INSTALL_DIR/game-deploy"
echo "Restart your shell or run: export PATH=\"$INSTALL_DIR:\$PATH\""
