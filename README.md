将gotify消息转发到bark等其他平台。

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

构建使用 [Taskfile](https://taskfile.dev)（命令为 `go-task` / `task`），依赖 Docker 和 `gomod-cap`（首次执行 `task download-tools` 安装）。
本仓库不提交 `Taskfile.yaml`，克隆后将以下内容保存为 `Taskfile.yaml` 即可编译：

```yaml
version: '3'

vars:
  BUILDDIR: ./build
  GOTIFY_VERSION: v3.1.0
  PLUGIN_NAME: gotify-plugin-forward
  DOCKER_BUILD_IMAGE: gotify/build
  DOCKER_WORKDIR: /proj

tasks:
  default:
    desc: 列出所有任务
    cmds:
      - task --list

  download-tools:
    desc: 安装依赖对齐工具 gomod-cap
    cmds:
      - go install github.com/gotify/plugin-api/cmd/gomod-cap@latest

  create-build-dir:
    desc: 创建构建目录
    cmds:
      - mkdir -p {{.BUILDDIR}}
    status:
      - test -d {{.BUILDDIR}}

  update-go-mod:
    desc: 按 gotify server {{.GOTIFY_VERSION}} 的 go.mod 对齐依赖
    deps: [create-build-dir]
    cmds:
      - wget -LO {{.BUILDDIR}}/gotify-server.mod https://raw.githubusercontent.com/gotify/server/{{.GOTIFY_VERSION}}/go.mod
      - $(go env GOPATH)/bin/gomod-cap -from {{.BUILDDIR}}/gotify-server.mod -to go.mod
      - rm -f {{.BUILDDIR}}/gotify-server.mod
      - go mod tidy

  get-gotify-server-go-version:
    desc: 获取 gotify server {{.GOTIFY_VERSION}} 的 Go 编译版本
    deps: [create-build-dir]
    cmds:
      - wget -LO {{.BUILDDIR}}/gotify-server-go-version https://raw.githubusercontent.com/gotify/server/{{.GOTIFY_VERSION}}/GO_VERSION

  build-linux-amd64:
    desc: 编译 linux-amd64 插件
    deps: [get-gotify-server-go-version, update-go-mod]
    cmds:
      - docker run --rm
        -v "$PWD:{{.DOCKER_WORKDIR}}"
        -v "$(go env GOPATH)/pkg/mod:/go/pkg/mod:ro"
        -w {{.DOCKER_WORKDIR}}
        {{.DOCKER_BUILD_IMAGE}}:$(cat {{.BUILDDIR}}/gotify-server-go-version)-linux-amd64
        go build -mod=readonly -a -installsuffix cgo -buildmode=plugin
        -o {{.BUILDDIR}}/{{.PLUGIN_NAME}}-linux-amd64.so {{.DOCKER_WORKDIR}}

  build-linux-arm-7:
    desc: 编译 linux-arm-7 插件
    deps: [get-gotify-server-go-version, update-go-mod]
    cmds:
      - docker run --rm
        -v "$PWD:{{.DOCKER_WORKDIR}}"
        -v "$(go env GOPATH)/pkg/mod:/go/pkg/mod:ro"
        -w {{.DOCKER_WORKDIR}}
        {{.DOCKER_BUILD_IMAGE}}:$(cat {{.BUILDDIR}}/gotify-server-go-version)-linux-arm-7
        go build -mod=readonly -a -installsuffix cgo -buildmode=plugin
        -o {{.BUILDDIR}}/{{.PLUGIN_NAME}}-linux-arm-7.so {{.DOCKER_WORKDIR}}

  build-linux-arm64:
    desc: 编译 linux-arm64 插件
    deps: [get-gotify-server-go-version, update-go-mod]
    cmds:
      - docker run --rm
        -v "$PWD:{{.DOCKER_WORKDIR}}"
        -v "$(go env GOPATH)/pkg/mod:/go/pkg/mod:ro"
        -w {{.DOCKER_WORKDIR}}
        {{.DOCKER_BUILD_IMAGE}}:$(cat {{.BUILDDIR}}/gotify-server-go-version)-linux-arm64
        go build -mod=readonly -a -installsuffix cgo -buildmode=plugin
        -o {{.BUILDDIR}}/{{.PLUGIN_NAME}}-linux-arm64.so {{.DOCKER_WORKDIR}}

  build:
    desc: 编译全平台插件（amd64 / arm-7 / arm64）
    deps: [build-linux-arm-7, build-linux-amd64, build-linux-arm64]
```
