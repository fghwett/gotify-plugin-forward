将gotify消息转发到bark等其他平台。

## 功能

- 消息推送接口与原版 gotify 兼容（`.../message?token=应用token`），可无缝替换原地址
- 支持按 token 配置转发规则，多个渠道并行推送，`all` 作为兜底规则
- 规则支持匹配条件：标题/内容关键字（可用正则）、排除词、消息优先级区间，
  同一 token 可配多条规则让不同消息走不同渠道
- 支持 Bark 推送：铃声、时效性等级（可按优先级自动推导）、分组、图标、
  点击跳转、历史存档、AES 端到端加密等参数均可配置
- 转发在消息入库后异步执行，不拖慢接口响应；单渠道失败自动重试（退避 0.5s / 1s）
- 投递日志持久化保存最近 200 条记录（时间、标题、渠道、状态、重试次数、错误原因），
  可在配置页按状态/渠道过滤查看
- 可视化配置页面：在插件详情页点击链接进入，使用 passkey（指纹 / 面容 / 硬件密钥）保护，
  支持渠道「测试发送」、配置导入导出、未保存提醒与表单校验

## 可视化配置

启用插件后，在插件详情页（Display 区域）可以看到配置页面地址，形如：

```
https://your-gotify.com/plugin/<插件ID>/custom/<随机令牌>/
```

- 首次打开时设置 passkey，之后每次进入都需要验证
- 页面分为四个标签：
  - **渠道**：管理 Bark 渠道与参数，保存前可用「测试发送」即时验证
  - **规则**：按 token 绑定渠道并配置匹配条件（关键字/正则/排除词/优先级区间）
  - **日志**：查看最近投递记录与失败原因，支持按状态/渠道过滤、自动刷新
  - **关于**：插件版本、配置导入导出、passkey 说明
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
