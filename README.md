将gotify消息转发到bark等其他平台。

## 源码使用

```shell
# 下载源码
git clone https://github.com/fghwett/gotify-plugin-forward.git

# 进入目录
cd gotify-plugin-forward

# 编译 使用对应平台的编译命令
make build-linux-amd64

# 上传插件
cp ./build/gotify-plugin-forward-linux-amd64 username@youserver.com:/gotify/data/plugins/

# 重启服务器
systemctl restart gotify-server

# 启用插件 之后后台操作即可
```
