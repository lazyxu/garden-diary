# 🌿 花园日记 / Garden Diary

与家人一起记录每株花的生长，从第一片叶子到每一次开花。轻量、自托管，照片始终存放在自己的服务器中。

## 已实现的首版功能

- **家庭共享花园**：首次初始化管理员、独立成员账号、管理员添加/禁用成员；所有家庭成员共同管理花卉，日记显示记录者。
- **花卉管理**：每一株独立档案；品种分类、搜索、种植日期、位置、备注、归档/恢复、删除。
- **成长日记**：手机直接拍照或选相册，每篇 1–8 张照片；拍摄时间可修改，备注可为空；支持编辑时间/备注、删除记录。
- **照片**：JPEG、PNG、HEIC 原图保留，生成缩略图，照片网格、全屏查看与原图下载。HEIC 缩略图及高清预览使用容器内 `heif-convert`；JPEG 缩略图按 EXIF 方向显示。
- **回顾**：单株成长时间线、花园日历（点击日期查看当天记录）、任选两张成长照片对比。
- **数据与管理**：SQLite + 本地照片，家庭数据 ZIP 导出，停写一致性备份、恢复、更新失败回滚、安装渠道。
- **响应式 Web + PWA**：电脑和手机使用相同后端，中文自然简约界面，可切换深色模式。

不在首版范围：养护提醒、离线写入、多家庭隔离、视频、公开分享链接和原生 iOS/Android App。

## 架构

- API: Go 1.26、标准库 HTTP、modernc SQLite 驱动（纯 Go）、bcrypt、服务器端会话 Cookie 和 CSRF 防护。
- Web: React 18 + TypeScript + MUI + Vite，Caddy 承载静态资源与 API 反向代理。
- 存储：`~/.garden-diary/data/app/garden.sqlite`、`originals/`、`thumbs/`。
- 部署：两个 Docker 容器（API 和 Web），可选第三个独立 HTTPS Caddy 服务；不暴露 Docker socket 给应用容器。

## 安装：Linux + Docker Compose

需要 Linux amd64/arm64、Docker Engine、Docker Compose v2、`curl`、`tar`、`flock`。请先确认当前用户可以执行 `docker info`。

**首次安装（master 渠道）：**下载完整脚本到本地后运行，勿直接把 curl 管道传给 bash。

    tmp="$(mktemp)"
    curl -fsSL --retry 3 https://raw.githubusercontent.com/lazyxu/garden-diary/master/deploy/install.sh -o "$tmp"
    bash "$tmp" --channel master
    rc=$?
    rm -f "$tmp"
    exit "$rc"

稳定版渠道在项目发布首个 `vX.Y.Z` Release 后启用，运行 `bash install.sh --channel stable`。安装器根据提交 SHA 或正式标签拉取对应 CI 构建的 GHCR 镜像。

默认访问：`http://服务器IP:3018`。第一次打开时，需要用安装器显示的 **GD_SETUP_TOKEN** 在网页创建首位管理员；口令也保存在宿主机 `~/.garden-diary/config/.env`（权限 0600）中，请勿公开。由管理员在“家庭”中给家人创建账号。

安装目录：

    ~/.garden-diary/
      bin/garden-diary
      bin/install.sh
      config/.env
      config/docker-compose.yml
      data/app/garden.sqlite
      data/app/originals/
      data/app/thumbs/
      data/caddy/
      backups/snapshots/
      backups/pre-upgrade/
      backups/pre-restore/
      state/install.lock

自定义 `GD_HOME` 可改变整个安装目录。更改端口前编辑 `config/.env` 的 `GD_HTTP_PORT`，然后在安装目录执行 Docker Compose 重建服务。

## 服务器管理

    ~/.garden-diary/bin/garden-diary status
    ~/.garden-diary/bin/garden-diary doctor
    ~/.garden-diary/bin/garden-diary logs
    ~/.garden-diary/bin/garden-diary backup
    ~/.garden-diary/bin/garden-diary restore /path/to/backup.tar.gz --yes
    ~/.garden-diary/bin/garden-diary update
    ~/.garden-diary/bin/garden-diary update --channel stable
    ~/.garden-diary/bin/garden-diary uninstall --yes
    ~/.garden-diary/bin/garden-diary uninstall --purge-data --yes

升级时先暂停 SQLite 写入，生成数据库与原图的一致性 tar.gz 快照；拉取固定提交/tag 的镜像并重建容器，待 API 和 Web 健康检查通过后才视为成功。失败时恢复旧配置、旧镜像和旧数据。快照存放在 `backups/pre-upgrade`，**不会随容器重建丢失**。正常卸载保留数据；清除数据必须显式传 `--purge-data --yes`。定期把备份复制到其他存储设备。

手动备份与恢复期间 API 会短暂不可用。ZIP 导出提供可迁移的原图和 JSON 清单，但不是直接可恢复的 SQLite 快照。

## 域名与 HTTPS（可选）

默认安装仅需局域网 IP，不需要域名。配置有效域名、DNS A/AAAA 解析和端口转发后运行：

    ~/.garden-diary/bin/garden-diary enable-https garden.example.com

Caddy 默认通过公网 80/443 验证并签发 HTTPS 证书。也可以修改 `GD_HTTPS_HTTP_PORT` / `GD_HTTPS_PORT` 并配合路由器做端口映射。关闭独立 HTTPS 服务：

    ~/.garden-diary/bin/garden-diary disable-https

**PWA 安全上下文限制：**普通局域网 IP 的 HTTP 可以浏览和上传记录，但移动浏览器通常要求 HTTPS 才允许安装 PWA 或运行 Service Worker。域名 HTTPS 是手机 PWA 的推荐入口；不将 HTTP 下无法安装 PWA 错报为应用故障。

## 本地开发

    # 终端 A
    export GD_DATA_DIR=./data
    export GD_SETUP_TOKEN=local-change-me
    go mod tidy
    go run ./cmd/garden-diary

    # 终端 B
    cd web
    npm install
    npm run dev

Vite 将 `/api/*` 转发到 `http://127.0.0.1:8080`。访问 `http://localhost:5173`。测试命令：

    go test ./...
    go vet ./...
    cd web && npm run build

## 安全及限制

应用不提供公共注册。首次管理员初始化必须提供部署时生成的口令；之后管理员再创建家人账号。登录 Cookie 为 HttpOnly、SameSite，API 写入操作还需 CSRF Token。HTTPS 反向代理后 Cookie 会使用 Secure 属性。照片 API 在鉴权后返回原图，不提供公开 URL。上传通过文件签名校验并限制单张 25 MiB、整篇 128 MiB。

当前所有成员拥有共享花园内容的编辑/删除权限；只有管理员能创建/禁用成员或导出全量数据。SQLite 采用 WAL，需将数据库及相关文件保存在同一持久目录中。HEIC 预览依赖服务器 `heif-convert` 支持的编码格式；如果转换失败，原文件仍被保存。

提交变更先运行 Go 后端测试、前端 TypeScript/Vite 构建与 Compose 校验。完整发布机制见 `.github/workflows/ci.yml`。
