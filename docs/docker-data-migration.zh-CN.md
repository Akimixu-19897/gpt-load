# Docker 数据迁移到另一台 Mac

本文档记录如何把当前这台 Mac 上的 `gpt-load` Docker 数据迁移到另一台 Mac，例如公司电脑。代码和镜像可以从 GitHub / GHCR 获取，但分组、密钥、配置、日志等运行数据在本机 Docker 数据目录里，需要单独迁移。

## 当前数据位置

当前本机部署使用的宿主机数据目录是：

```bash
/Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001
```

容器启动时会把该目录挂载到容器内：

```bash
/app/data
```

只要另一台 Mac 启动容器时也把迁移后的数据目录挂载到 `/app/data`，就可以复用原来的配置和数据。

## 重要安全提醒

这个数据目录可能包含：

- 管理后台配置
- 分组配置
- 上游地址
- API key 数据
- 请求日志
- SQLite 数据库文件

不要把数据压缩包提交到 Git，也不要上传到公开网盘或公开仓库。建议只放到你自己可控的私有云盘，并在迁移完成后删除不再需要的压缩包。

## 在当前 Mac 打包数据

先停止容器，避免打包时数据库还在写入：

```bash
cd /Users/akimixu/Desktop/Projects/gpt-load

docker stop gpt-load
```

打包数据目录：

```bash
tar -czf gpt-load-data-ghcr-3001.$(date +%Y%m%d-%H%M%S).tar.gz data-ghcr-3001
```

打包完成后可以重新启动当前 Mac 上的容器：

```bash
docker start gpt-load
```

确认压缩包已生成：

```bash
ls -lh gpt-load-data-ghcr-3001.*.tar.gz
```

然后把生成的 `.tar.gz` 文件上传到你的私有云盘。

## 在公司 Mac 恢复数据

先准备一个本地目录，例如：

```bash
mkdir -p ~/Projects/gpt-load
cd ~/Projects/gpt-load
```

把云盘下载下来的压缩包放到这个目录，然后解压：

```bash
tar -xzf gpt-load-data-ghcr-3001.实际时间戳.tar.gz
```

解压后应该能看到：

```bash
ls -la data-ghcr-3001
```

## 在公司 Mac 启动容器

拉取当前个人 GHCR 镜像：

```bash
docker pull ghcr.io/akimixu-19897/gpt-load:v1.4.7-akimixu.2
```

用迁移后的数据目录启动容器：

```bash
docker run -d \
  --name gpt-load \
  --restart unless-stopped \
  -p 3001:3001 \
  -e PORT=3001 \
  -e HOST=0.0.0.0 \
  -e TZ=Asia/Shanghai \
  -e AUTH_KEY=sk-ghcr-c6b45cbcadea3324 \
  -v ~/Projects/gpt-load/data-ghcr-3001:/app/data \
  ghcr.io/akimixu-19897/gpt-load:v1.4.7-akimixu.2
```

如果公司 Mac 的 `3001` 端口已经被占用，可以改用 `3002`：

```bash
docker run -d \
  --name gpt-load \
  --restart unless-stopped \
  -p 3002:3001 \
  -e PORT=3001 \
  -e HOST=0.0.0.0 \
  -e TZ=Asia/Shanghai \
  -e AUTH_KEY=sk-ghcr-c6b45cbcadea3324 \
  -v ~/Projects/gpt-load/data-ghcr-3001:/app/data \
  ghcr.io/akimixu-19897/gpt-load:v1.4.7-akimixu.2
```

此时访问地址对应为：

```bash
http://localhost:3002
```

## 验证迁移结果

检查容器状态：

```bash
docker ps --filter name=gpt-load
```

检查健康状态：

```bash
curl http://localhost:3001/health
```

如果使用的是 `3002`：

```bash
curl http://localhost:3002/health
```

检查当前镜像：

```bash
docker inspect gpt-load --format '{{.Config.Image}}'
```

检查数据挂载：

```bash
docker inspect gpt-load --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'
```

检查开机自启策略：

```bash
docker inspect gpt-load --format '{{.HostConfig.RestartPolicy.Name}}'
```

预期输出：

```bash
unless-stopped
```

预期挂载结果应类似：

```bash
/Users/你的用户名/Projects/gpt-load/data-ghcr-3001 -> /app/data
```

## 常见问题

### 端口被占用

如果启动时报端口占用，把 `-p 3001:3001` 改成 `-p 3002:3001`，然后访问 `http://localhost:3002`。

### 容器名已存在

如果公司 Mac 已经有同名容器：

```bash
docker stop gpt-load
docker rm gpt-load
```

注意不要使用 `docker rm -v gpt-load`，避免误删 Docker volume。

### 登录 key 是什么

当前迁移命令使用的管理 key 是：

```bash
sk-ghcr-c6b45cbcadea3324
```

如果你想在公司 Mac 使用新的管理 key，可以修改 `docker run` 里的 `AUTH_KEY`。但如果数据里已有相关配置或你希望两台机器保持一致，建议先沿用当前 key，迁移完成后再统一调整。

## 迁移完成后的清理

确认公司 Mac 可正常访问后，可以删除临时压缩包：

```bash
rm gpt-load-data-ghcr-3001.实际时间戳.tar.gz
```

如果云盘不需要长期保存这份数据备份，也建议删除云盘里的压缩包，避免 API key 数据长期暴露在额外位置。
