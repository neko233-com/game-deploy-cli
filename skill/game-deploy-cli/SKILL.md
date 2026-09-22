---
name: game-deploy-cli
description: 使用 game-deploy CLI 对游戏 Android Google Play 和 iOS App Store Connect 包体执行预检、计划和发布。
---

# game-deploy-cli

## 工作顺序

1. 先执行 `game-deploy --json doctor` 和 `game-deploy --json auth status`。
2. 发布前执行对应 provider 的 `--dry-run`，确认 artifact、包名/Bundle ID、track 和平台边界。
3. 只有用户明确要求发布，并且 dry-run 结果符合预期时，才添加 `--yes`。
4. 读取 JSON 结果中的 `status`、`message` 和错误退出码；不要把命令排队或本地校验当成发布成功。

## Android Google Play

```sh
game-deploy --json publish google-play \
  --artifact ./build/game.aab \
  --package com.example.game \
  --track internal \
  --dry-run
```

真实上传需要 `GOOGLE_PLAY_SERVICE_ACCOUNT_FILE` 或 `--service-account`，并且服务账号必须具有 Google Play Console 对目标应用的发布权限。

## iOS App Store Connect

```sh
game-deploy --json publish app-store-connect \
  --artifact ./build/game.ipa \
  --bundle-id com.example.game \
  --dry-run
```

真实 IPA 上传必须运行在安装了 Xcode Transporter 的 macOS，并配置 `ASC_API_KEY_FILE`、`ASC_KEY_ID`、`ASC_ISSUER_ID`。Windows/Linux 只能生成计划，不能报告真实上传成功。

## 安全约束

- 不在 prompt、日志、命令行参数或 Git 中写入私钥、service-account JSON 或 token。
- 不跳过 `--dry-run` 和 `--yes` 的确认边界。
- 遇到上传结果未知时，先查询平台状态，不更换参数盲目重试。
