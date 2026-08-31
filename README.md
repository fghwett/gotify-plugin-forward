将gotify消息转发到bark等其他平台。

## 功能

- 消息推送接口与原版 gotify 兼容（`.../message?token=应用token`），可无缝替换原地址
- 支持按 token 配置转发规则，多个渠道并行推送，`all` 作为兜底规则
- 支持 Bark 推送，可选 AES 加密（端到端加密推送）
- 可视化配置页面：在插件详情页点击链接进入，使用 passkey（指纹 / 面容 / 硬件密钥）保护

## 可视化配置

启用插件后，在插件详情页（Display 区域）可以看到配置页面地址，形如：

```
https://your-gotify.com/plugin/<插件ID>/custom/<随机令牌>/config
```

- 首次打开时设置 passkey，之后每次进入都需要验证
- 页面地址含随机令牌、本身私密，请妥善保存
- passkey 需要浏览器安全上下文（HTTPS 或 localhost）
- passkey 丢失时，在 gotify 原生插件配置中将 `reset_passkey` 设为 `true` 并保存，
  即可重新设置（完成后请改回 `false`）
- 可视化保存的配置优先生效；如从未保存过，则回落使用 gotify 原生 YAML 配置

## 源码使用

```shell
# 下载源码
git clone https://github.com/fghwett/gotify-plugin-forward.git

# 进入目录
cd gotify-plugin-forward

# 编译 使用对应平台的编译命令
task build-linux-amd64

# 上传插件
cp ./build/gotify-plugin-forward-linux-amd64.so username@youserver.com:/gotify/data/plugins/

# 重启服务器
systemctl restart gotify-server

# 启用插件 之后后台操作即可
```

构建使用 [Taskfile](https://taskfile.dev)（命令为 `go-task` / `task`），任务定义见仓库中的 `Taskfile.yaml`。
依赖 Docker、`gomod-cap`（首次执行 `task download-tools` 安装），
以及 Node.js + pnpm（用于构建 SolidJS 编写的可视化页面，`task ui-build`）。

## 版本发布

```shell
# 发布新版本（更新版本号、打 tag 并推送，自动触发 GitHub Actions）
task release VERSION=0.0.3
```

推送 `v*` 标签后 [Release workflow](.github/workflows/release.yml) 会自动：

1. 获取 gotify/server 的**最新**发布版本
2. 对齐依赖并编译 linux-amd64 / linux-arm64 两个平台的 `.so`
3. 校验产物与官方 gotify 镜像的共享依赖完全一致（防止发布无法加载的插件）
4. 创建 GitHub Release 并上传产物

另外还有 [Build workflow](.github/workflows/build.yml)，可在 Actions 页面手动触发，
**指定任意 gotify 版本**打包（产物以 artifact 形式下载，不发布 Release）。

> 若 gotify 新版本升级了共享依赖，Release 的兼容性校验会失败，
> 需要按 go.mod 中 replace 的注释更新钉住的版本后重新发版。
