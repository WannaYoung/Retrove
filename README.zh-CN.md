# Retrove

[English](README.md)

面向复古游戏收藏的自托管 ROM 库管理器：扫描本地目录、按内容 hash 识别、刮削元数据与媒体、在 Web 管理台维护中英标题，并导出到 Pegasus 等前端。

对标 RomM 一类工具的「扫描 → 识别 → 刮削 → 管理 → 导出」，把**中文名**和国内常见前端导出作为一等公民。

> Retrove **不包含、不分发** ROM。请自行提供有权使用的文件。

## 功能

- 多机型 ROM 库（FC / SFC / MD / FBNeo / MAME 等）
- 以内容 hash 作为稳定身份（改名不丢元数据）：主机算包内 ROM；街机算 zip 本身
- 一文件一条游戏（美版 / 日版 / 汉化 / Hack 各自独立）
- 天马导入为主：中文名、简介、开发商、box、logo、预览视频可反复导入替换
- 翻译把中文名写成英文名。刮削（IGDB → ScreenScraper → SteamGridDB）只补空位，不替换已有字段和媒体
- Web 管理台：浏览、编辑单款并写回天马 ROM 名 / 元数据 / 媒体目录；多主题与中英文界面
- 导出 Pegasus 等前端布局（详见文档）
- 支持 Docker / Compose；Go 单二进制 + Vue 管理端

## 技术栈

| 层级 | 选型 |
|------|------|
| 后端 | Go + SQLite（WAL） |
| 前端 | Vue 3 + TypeScript + Vite + Naive UI |
| 部署 | Docker / Docker Compose |

## 快速开始

### 环境要求

- Go（版本见 `go.mod`）
- Node.js 20+（前端）
- 可选：Docker + Docker Compose

### 本地开发

```bash
# 创建运行时目录；若无配置则从示例复制
make config

# 填写密钥（切勿提交此文件）
cp app/config/config.example.yaml app/config/config.yaml
# 按需填写 ScreenScraper / IGDB / SteamGridDB / LLM

# 终端 1 — API
make dev-backend

# 终端 2 — 管理台
make dev-frontend
```

- API 健康检查：http://127.0.0.1:5080/api/v1/health  
- 管理台：http://127.0.0.1:5180  

将 ROM 放入 `app/roms/<平台目录>/`（目录名与平台的映射见 `app/config/platforms.yaml`）。

### Docker

```bash
make config
# 编辑 app/config/config.yaml 后：
make docker-up
```

Compose 会挂载 `./app/config`、`./app/data`、`./app/roms`。默认端口：**5080**。

## 配置

| 路径 | 用途 |
|------|------|
| `app/config/config.example.yaml` | 仓库内示例模板 |
| `app/config/config.yaml` | 本地密钥（已 gitignore） |
| `app/config/platforms.yaml` | 目录名 ↔ 平台 / ScreenScraper system id / IGDB platform id |
| `app/config/dictionaries/` | 中文词典导入 / 备份 |
| `app/config/retrove.db` | SQLite 库（已 gitignore） |
| `app/data/` | 刮削媒体、缓存、导出 |
| `app/roms/` | 你的 ROM 树 |

刮削密钥只放在 `config.yaml`（或环境变量覆盖）。某源留空即禁用，不影响其他功能。

## 文档

- [开发文档](docs/开发文档.md)
- [开发清单](docs/开发清单.md)
- [部署指南](docs/部署指南.md)

## 目录结构

```text
retrove/
├── cmd/retrove/       # Go 入口
├── internal/          # 后端业务
├── web/               # Vue 管理台
├── app/               # 仅运行时（config / data / roms）
├── docs/              # 设计与部署（中英）
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

## 许可

仓库尚未附带正式 License 文件。分发或二次发布前请与维护者确认许可；不要把第三方 API 开发者凭据硬编码进公开镜像。
