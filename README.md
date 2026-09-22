# game-deploy-cli

面向 Agent 和游戏团队的跨平台发布 CLI。它只安装当前机器需要的单一二进制，不要求用户下载全平台 CLI，也不把凭据写入仓库或命令行参数。

当前 provider：

- `google-play`：上传 `.aab` 或 `.apk`，更新 `internal / alpha / beta / production` track，并提交 Google Play edit。
- `app-store-connect`：在 macOS 上使用 Xcode `iTMSTransporter` 上传 `.ipa`；Windows/Linux 支持生成计划和校验，但不能代替 Apple 的 macOS Transporter。

## 安装

POSIX shell（Linux/macOS）：

```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/game-deploy-cli/main/install.sh | sh
```

PowerShell（Windows）：

```powershell
irm https://raw.githubusercontent.com/neko233-com/game-deploy-cli/main/install.ps1 | iex
```

安装器从 GitHub Release 读取当前 OS/架构的 asset：
`game-deploy-{linux|darwin|windows}-{amd64|arm64}`，不会下载其它平台的包。可以用 `GAME_DEPLOY_VERSION` 固定版本，用 `GAME_DEPLOY_INSTALL_DIR` 改安装目录。

## Agent-friendly 使用

所有写操作都要求明确的 `--yes`；Agent 应先调用 `--dry-run`，读取 JSON 计划后再提交。`--json` 输出稳定 JSON，错误返回非零退出码。

```sh
game-deploy --json version
game-deploy --json doctor
game-deploy --json auth status

game-deploy --json publish google-play \
  --artifact ./build/game.aab \
  --package com.example.game \
  --track internal \
  --dry-run

game-deploy --json publish app-store-connect \
  --artifact ./build/game.ipa \
  --bundle-id com.example.game \
  --dry-run
```

真实发布：

```sh
game-deploy publish google-play \
  --artifact ./build/game.aab \
  --package com.example.game \
  --track internal \
  --service-account ./secrets/google-play-service-account.json \
  --yes

# macOS + Xcode
game-deploy publish app-store-connect \
  --artifact ./build/game.ipa \
  --bundle-id com.example.game \
  --api-key "$HOME/.appstoreconnect/private_keys/AuthKey_KEYID.p8" \
  --key-id KEYID \
  --issuer-id ISSUER_UUID \
  --yes
```

建议使用环境变量承载 CI 凭据路径：

```text
GOOGLE_PLAY_SERVICE_ACCOUNT_FILE=/protected/google-play-service-account.json
ASC_API_KEY_FILE=/protected/AuthKey_KEYID.p8
ASC_KEY_ID=KEYID
ASC_ISSUER_ID=ISSUER_UUID
```

不要把 service-account JSON、`.p8` 私钥或任何 token 提交到 Git。`auth status` 只报告是否配置，不输出凭据内容。

## 构建与验证

```sh
go test ./...
go build -trimpath -o bin/game-deploy ./cmd/game-deploy
bin/game-deploy --json publish google-play --artifact ./game.aab --package com.example.game --dry-run
```

## 设计边界

Google Play 使用官方 Android Publisher REST API：创建 edit、上传包体、更新 track、提交 edit。iOS 的 IPA 上传由 Apple 官方 Transporter 完成，CLI 只负责统一参数、校验、确认门和进程调用；这避免在非 macOS 环境伪造“发布成功”。后续可在同一 provider 接口下增加商店元数据、测试组和渠道配置，而不改变安装器协议。

命令形态参考 [TapTap CLI 文档](https://developer.taptap.cn/v3/cli/)，但安装策略改为单平台 release asset，发布写操作采用显式确认。
