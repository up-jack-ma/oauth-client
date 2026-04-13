# OAuth Client

Go + React + SQLite 的多 OAuth 提供商登录客户端，支持 Docker 部署。

## 功能

- 标准 OAuth 2.0 Authorization Code 流程
- 支持多家 OAuth 提供商 (GitHub、Google、GitLab、Discord 等)
- 用户登录后展示已关联的第三方账户
- 管理后台 (`/admin`)：配置 OAuth 提供商、管理用户
- 内置预设模板，快速添加常见提供商
- Docker 多阶段构建，最终镜像 ~20MB
- SQLite 存储，无需外部数据库

## 架构

```
backend/          Go 后端 (Gin + SQLite)
  ├── main.go           入口
  ├── handlers/         API 处理器
  ├── middleware/        JWT 认证 + 权限
  ├── models/           数据模型
  └── database/         数据库初始化 + 迁移
frontend/         React 前端 (Vite)
  └── src/
      ├── pages/        LoginPage, AccountsPage, AdminPage
      └── api.js        API 客户端
docker/           Nginx 配置
```

## 快速开始

### Docker 部署（推荐）

```bash
# 1. 克隆项目
git clone <repo-url> && cd oauth-client

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env，设置 BASE_URL 和 JWT_SECRET

# 3. 构建并启动
docker compose up -d

# 访问 http://localhost:8080
```

### 带 Nginx + SSL

```bash
# 1. 修改 docker/nginx.conf 中的域名
# 2. 放置 SSL 证书到 docker/certs/ 目录
#    - fullchain.pem
#    - privkey.pem

# 3. 启动（含 Nginx）
docker compose --profile with-nginx up -d
```

### 使用 Let's Encrypt

```bash
# 在 VPS 上安装 certbot
sudo apt install certbot

# 获取证书
sudo certbot certonly --standalone -d your-domain.com

# 复制证书
cp /etc/letsencrypt/live/your-domain.com/fullchain.pem docker/certs/
cp /etc/letsencrypt/live/your-domain.com/privkey.pem docker/certs/
```

## 本地开发

```bash
# 后端
cd backend && go run .

# 前端（另一个终端）
cd frontend && npm install && npm run dev
```

## 默认管理员账号

首次启动会自动创建管理员：

- Email: `admin@example.com`
- Password: `admin123`

**请登录后立即修改密码！**

## 配置 OAuth 提供商

1. 登录管理员账号
2. 访问 `/admin`
3. 点击 "Quick Add" 选择预设或 "Custom Provider" 手动添加
4. 填入在第三方平台获取的 Client ID 和 Client Secret
5. Callback URL 格式: `https://your-domain.com/api/auth/{provider-name}/callback`

### 各平台创建应用指引

| 提供商 | 创建应用地址 | Callback URL |
|--------|-------------|--------------|
| GitHub | https://github.com/settings/developers | `https://域名/api/auth/github/callback` |
| Google | https://console.cloud.google.com/apis/credentials | `https://域名/api/auth/google/callback` |
| GitLab | https://gitlab.com/-/user_settings/applications | `https://域名/api/auth/gitlab/callback` |
| Discord | https://discord.com/developers/applications | `https://域名/api/auth/discord/callback` |

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `BASE_URL` | 服务公开访问地址 | `http://localhost:8080` |
| `JWT_SECRET` | JWT 签名密钥 | `change-me-in-production` |
| `PORT` | 监听端口 | `8080` |
| `DB_PATH` | SQLite 数据库路径 | `./data/oauth.db` |
| `GIN_MODE` | Gin 运行模式 | `release` |

## API 路由

```
公开:
  GET  /api/providers                    获取启用的提供商列表
  GET  /api/auth/:provider               发起 OAuth 登录
  GET  /api/auth/:provider/callback      OAuth 回调
  POST /api/auth/login                   邮箱密码登录
  POST /api/auth/register                注册

需登录:
  GET  /api/me                           获取当前用户信息
  GET  /api/accounts                     获取关联账户列表
  POST /api/auth/:provider/link          关联新的 OAuth 账户
  DELETE /api/accounts/:id               取消关联

管理后台:
  GET  /api/admin/providers              获取所有提供商（含密钥）
  POST /api/admin/providers              创建提供商
  PUT  /api/admin/providers/:id          更新提供商
  DELETE /api/admin/providers/:id        删除提供商
  GET  /api/admin/users                  获取用户列表
  PUT  /api/admin/users/:id/role         修改用户角色
  GET  /api/admin/stats                  获取统计数据
```
