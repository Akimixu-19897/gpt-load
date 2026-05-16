# Docker 容器安全更新流程

本文档记录当前本机 `gpt-load` Docker 部署的安全更新方式。目标是在更新镜像和重建容器时保留已有数据。

## 当前部署信息

| 项目 | 当前值 |
| --- | --- |
| 容器名 | `gpt-load` |
| 服务端口 | `3001` |
| 镜像仓库 | `ghcr.io/akimixu-19897/gpt-load` |
| 容器数据目录 | `/app/data` |
| 宿主机数据目录 | `/Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001` |
| 当前管理密钥 | `sk-ghcr-c6b45cbcadea3324` |

## 数据保留原则

Docker 容器可以删除和重建，镜像也可以更新，但宿主机数据目录不能删除：

```bash
/Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001
```

每次启动新容器时，都必须继续把这个目录挂载到容器的 `/app/data`：

```bash
-v /Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001:/app/data
```

只要这个挂载关系保持不变，管理后台配置、数据库和运行数据都会继续保留。

## 标准更新命令

将下面命令里的 `目标版本号` 替换为要部署的新镜像 tag，例如 `v1.4.7-akimixu.1`。

```bash
cd /Users/akimixu/Desktop/Projects/gpt-load

# 1. 备份当前数据目录
cp -a data-ghcr-3001 "data-ghcr-3001.backup.$(date +%Y%m%d-%H%M%S)"

# 2. 拉取新镜像
docker pull ghcr.io/akimixu-19897/gpt-load:目标版本号

# 3. 停止并删除旧容器
# 注意：这里没有使用 -v，不会删除宿主机数据目录
docker stop gpt-load
docker rm gpt-load

# 4. 使用同一个数据目录启动新容器
docker run -d \
  --name gpt-load \
  -p 3001:3001 \
  -e PORT=3001 \
  -e HOST=0.0.0.0 \
  -e TZ=Asia/Shanghai \
  -e AUTH_KEY=sk-ghcr-c6b45cbcadea3324 \
  -v /Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001:/app/data \
  ghcr.io/akimixu-19897/gpt-load:目标版本号

# 5. 检查健康状态
curl http://localhost:3001/health

# 6. 查看启动日志
docker logs --tail 80 gpt-load
```

## 回滚到上一个备份

如果新版本启动失败，可以先停止容器，再恢复备份目录。

```bash
cd /Users/akimixu/Desktop/Projects/gpt-load

docker stop gpt-load
docker rm gpt-load

# 将 BACKUP_DIR 替换为实际备份目录名
mv data-ghcr-3001 "data-ghcr-3001.failed.$(date +%Y%m%d-%H%M%S)"
cp -a BACKUP_DIR data-ghcr-3001

docker run -d \
  --name gpt-load \
  -p 3001:3001 \
  -e PORT=3001 \
  -e HOST=0.0.0.0 \
  -e TZ=Asia/Shanghai \
  -e AUTH_KEY=sk-ghcr-c6b45cbcadea3324 \
  -v /Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001:/app/data \
  ghcr.io/akimixu-19897/gpt-load:目标版本号
```

## 禁止执行的危险命令

下面这些命令可能删除容器数据或备份，请谨慎使用：

```bash
docker rm -v gpt-load
docker volume prune
rm -rf /Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001
rm -rf /Users/akimixu/Desktop/Projects/gpt-load/data-ghcr-3001.backup.*
```

## 日常检查命令

```bash
# 查看容器状态
docker ps --filter name=gpt-load

# 查看当前镜像版本
docker inspect gpt-load --format '{{.Config.Image}}'

# 查看数据挂载
docker inspect gpt-load --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'

# 查看健康状态
curl http://localhost:3001/health
```
