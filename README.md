# 证券交易系统 (Stock Trading System)

一个完整的、高性能的生产级证券交易系统，采用DDD领域驱动设计，支持股票买卖订单管理、实时行情撮合、资金持仓管理、对账风控等核心功能。

## 目录

- [项目概述](#项目概述)
- [DDD架构](#ddd架构)
- [微服务架构](#微服务架构)
- [核心功能](#核心功能)
- [快速开始](#快速开始)
- [API文档](#api文档)
- [一键部署](#一键部署)
- [项目结构](#项目结构)
- [极端情况处理](#极端情况处理)
- [监控运维](#监控运维)

## 项目概述

本系统是一个完整的证券交易平台，采用微服务架构和DDD领域驱动设计，具有以下特点：

- **DDD架构**: 清晰的四层架构设计，领域模型与业务逻辑分离
- **高性能**: 采用Go语言开发，支持高并发、低延迟交易撮合
- **高可用**: 微服务架构，支持水平扩展和故障转移
- **资金安全**: TCC分布式事务确保资金安全
- **完整风控**: 多层次风控体系，防止异常交易
- **对账系统**: 自动对账检测和修复资金差异

## DDD架构

### 架构设计原则

本系统采用DDD（Domain-Driven Design）领域驱动设计，将业务逻辑清晰分层：

```
┌─────────────────────────────────────────────────────────────┐
│                      cmd/main.go                            │
│                      应用入口层                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/interfaces/                      │
│                      接口层 (Interface)                      │
│         gRPC处理器  │  HTTP处理器  │  WebSocket处理器        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/application/                      │
│                    应用层 (Application)                      │
│                      应用服务                                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/domain/                          │
│                     领域层 (Domain)                          │
│    聚合根  │  领域实体  │  领域服务  │  值对象  │  仓储接口   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 internal/infrastructure/                     │
│                    基础设施层 (Infrastructure)               │
│   配置  │  数据库  │  日志  │  消息  │  仓储实现  │  安全    │
└─────────────────────────────────────────────────────────────┘
```

### 各层职责

| 层次 | 职责 | 包含内容 |
|------|------|----------|
| **cmd/main.go** | 应用入口 | 依赖注入、服务启动、信号处理 |
| **interfaces** | 接口层 | gRPC/HTTP/WebSocket协议转换、请求路由 |
| **application** | 应用层 | 用例编排、事务管理、跨聚合操作 |
| **domain** | 领域层 | 聚合根、实体、值对象、领域服务、仓储接口 |
| **infrastructure** | 基础设施层 | 数据库访问、日志、消息队列、配置、外部服务 |

### 服务DDD结构示例

以账户服务为例的完整DDD结构：

```
backend/services/account-service/
├── cmd/main.go                         ├── config.yaml                          # 应用入口
 # 配置文件
└── internal/
    ├── domain/                          # 领域层
    │   ├── entity/
    │   │   └── account.go               # 账户、持仓、资金锁定聚合根
    │   │                               # - Account (账户聚合根)
    │   │                               # - Position (持仓聚合根)
    │   │                               # - FundLock (资金锁定)
    │   │                               # - TCCTransaction (分布式事务)
    │   ├── service/
    │   │   └── account_domain_service.go # 领域服务
    │   │                               # - CreateAccount
    │   │                               # - Deposit/Withdraw
    │   │                               # - LockFunds/UnlockFunds
    │   │                               # - SettleTrade
    │   │                               # - Reconcile
    │   └── repository/
    │       └── repository.go            # 仓储接口定义
    │                                   # - AccountRepository
    │                                   # - PositionRepository
    │                                   # - FundLockRepository
    │                                   # - TCCTransactionRepository
    ├── infrastructure/                  # 基础设施层
    │   ├── config/
    │   │   └── config.go               # 配置加载
    │   ├── database/
    │   │   └── mysql.go                # MySQL连接管理
    │   ├── logger/
    │   │   └── logger.go               # 结构化日志组件
    │   ├── messaging/
    │   │   └── publisher.go            # Redis消息发布/订阅
    │   └── repository/
    │       └── account_repository.go   # 仓储实现
    │                                   # - PostgresAccountRepository
    │                                   # - PostgresPositionRepository
    └── interfaces/                      # 接口层
        ├── grpc/
        │   └── server.go               # gRPC处理器
        │                               # - CreateAccount
        │                               # - GetAccount
        │                               # - Deposit/Withdraw
        │                               # - LockFunds/UnlockFunds
        │                               # - Reconcile
        └── http/
            └── server.go               # HTTP REST API处理器
                                        # - GET /api/v1/accounts/:user_id
                                        # - POST /api/v1/accounts/deposit
                                        # - POST /api/v1/accounts/withdraw
```

### 所有服务DDD结构统计

| 服务 | 领域实体 | 领域服务 | 仓储接口 | 基础设施 | 接口协议 |
|------|----------|----------|----------|----------|----------|
| user-service | User, Account | UserDomainService | UserRepository | config, logger, security | gRPC, HTTP |
| order-service | Order, TCCTransaction | OrderDomainService | OrderRepository, TCCRepository | config, database, logger, messaging | gRPC, HTTP |
| account-service | Account, Position, FundLock, TCCTransaction | AccountDomainService | AccountRepository, PositionRepository | config, database, logger, messaging | gRPC, HTTP |
| market-service | Quote, KLine, Symbol | MarketDomainService | QuoteRepository, KLineRepository, SymbolRepository | config, logger, messaging | gRPC, HTTP, WebSocket |
| matching-service | Order, OrderBook, Trade | MatchingEngine | OrderBookRepository | config, database, logger, messaging | gRPC, HTTP, WebSocket |
| api-gateway | GatewayRoute, ServiceEndpoint | GatewayDomainService | - | config, logger | HTTP |
| reconciliation-service | Discrepancy, ReconciliationReport, FixRecord | ReconciliationDomainService | DiscrepancyRepository, ReportRepository, FixRecordRepository | config, logger | HTTP |

## 微服务架构

### 系统架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           客户端 (Browser/Mobile)                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    API Gateway (:8080)                                   │
│         统一入口 │ 路由转发 │ 认证授权 │ 限流熔断 │ 日志追踪              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
        ┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
        │   User Service    │ │  Order Service    │ │  Account Service  │
        │   (:5001/:8081)   │ │  (:5002/:8082)    │ │  (:5004/:8084)    │
        │                   │ │                   │ │                   │
        │ 用户注册/登录      │ │ 订单CRUD          │ │ 账户余额管理       │
        │ 身份认证(JWT)     │ │ TCC事务管理       │ │ 资金冻结/解冻      │
        │ 用户信息管理       │ │ 频率限制          │ │ 持仓管理           │
        └───────────────────┘ └───────────────────┘ └───────────────────┘
                    │               │               │
                    │               │               │
                    ▼               ▼               ▼
        ┌─────────────────────────────────────────────────────────────────┐
        │                      消息总线 (Kafka)                            │
        │    order_events │ account_events │ market_data │ trade_events   │
        └─────────────────────────────────────────────────────────────────┘
                    │               │               │
                    ▼               ▼               ▼
        ┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
        │  Market Service   │ │Matching Service   │ │Reconciliation     │
        │   (:5003/:8083)   │ │  (:5005/:8085)    │ │  (:5006/:8086)    │
        │                   │ │                   │ │                   │
        │ 实时行情(K线/报价) │ │ 价格优先时间优先   │ │ 资金对账           │
        │ WebSocket推送     │ │ 撮合引擎          │ │ 持仓对账           │
        │ 盘口数据          │ │ 成交记录          │ │ 差异检测/修复      │
        └───────────────────┘ └───────────────────┘ └───────────────────┘
                                    │
                                    ▼
        ┌─────────────────────────────────────────────────────────────────┐
        │                        中间件服务                                │
        │                                                                   │
        │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
        │  │  MySQL   │  │  Redis   │  │  Kafka   │  │  Consul  │        │
        │  │ (:3306)  │  │ (:6379)  │  │ (:9092)  │  │ (:8500)  │        │
        │  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
        │                                                                   │
        └─────────────────────────────────────────────────────────────────┘
```

### 服务端口分配

| 服务 | gRPC端口 | HTTP端口 | 健康检查端点 | 主要功能 |
|------|----------|----------|--------------|----------|
| API Gateway | - | 8080 | /health | 统一入口、路由、认证 |
| User Service | 5001 | 8081 | /ready | 用户管理、认证 |
| Order Service | 5002 | 8082 | /ready | 订单管理、TCC事务 |
| Account Service | 5004 | 8084 | /ready | 资金管理、持仓管理 |
| Market Service | 5003 | 8083 | /ready | 行情数据、K线、WebSocket |
| Matching Service | 5005 | 8085 | /ready | 订单撮合引擎 |
| Reconciliation | 5006 | 8086 | /ready | 对账服务、差异修复 |
| MySQL | - | 3306 | - | 主数据库 |
| Redis | - | 6379 | - | 缓存、消息发布 |
| Kafka | - | 9092 | - | 事件流 |

### 服务通信模式

1. **同步通信 (gRPC/HTTP)**
   - 实时性要求高的操作
   - API Gateway → 微服务
   - 订单创建、资金锁定等

2. **异步通信 (Kafka)**
   - 事件驱动的业务流程
   - 订单状态变更通知
   - 行情数据分发
   - 对账任务触发

## 核心功能

### 交易功能
- 限价单、市价单支持
- 订单创建、查询、取消
- 部分成交、完全成交
- 订单状态跟踪

### 行情功能
- 实时行情报价
- K线数据(1分钟/5分钟/15分钟/1小时/1日)
- 盘口数据(买卖档位)
- WebSocket实时推送

### 资金功能
- 账户余额管理
- 资金冻结/解冻(TCC模式)
- 入金/出金
- 持仓管理

### 风控功能
- 订单频率限制
- 金额限制检查
- 价格偏离检测
- 并发安全控制

### 对账功能
- 资金对账
- 持仓对账
- 交易对账
- 自动差异修复

## 快速开始

### 环境要求
- Docker 20.10+
- Docker Compose 2.0+
- Git
- 内存: 最低4GB，推荐8GB+
- 磁盘: 最低20GB

### 1. 克隆项目

```bash
git clone https://github.com/yourusername/stock_trader.git
cd stock_trader
```

### 2. 一键部署

```bash
cd deployment

# 完整部署（检查环境、备份、构建、启动、验证）
./deploy.sh deploy

# 仅构建镜像
./deploy.sh build

# 启动服务
./deploy.sh start

# 查看服务状态
./deploy.sh status

# 运行健康检查
./deploy.sh health
```

### 3. 验证部署

```bash
# 检查所有服务健康状态
./deploy.sh health

# 查看服务日志
./deploy.sh logs api-gateway

# 查看实时日志
docker-compose logs -f
```

### 4. 访问系统

- **API Gateway**: http://localhost:8080
- **健康检查**: http://localhost:8080/health

## API文档

### 认证接口

#### 用户注册
```bash
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "test_user",
  "password": "password123",
  "email": "user@example.com"
}
```

#### 用户登录
```bash
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "test_user",
  "password": "password123"
}
```

响应:
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user_id": "uuid-xxx"
}
```

### 订单接口

#### 创建订单
```bash
POST /api/v1/orders
Authorization: Bearer <token>
Content-Type: application/json

{
  "symbol": "600519",
  "order_type": "LIMIT",
  "side": "BUY",
  "price": "1800.00",
  "quantity": 100,
  "client_order_id": "optional-id"
}
```

#### 查询订单
```bash
GET /api/v1/orders/:order_id
Authorization: Bearer <token>
```

#### 取消订单
```bash
POST /api/v1/orders/:order_id/cancel
Authorization: Bearer <token>
```

#### 查询用户订单
```bash
GET /api/v1/users/:user_id/orders?status=PENDING&limit=50
Authorization: Bearer <token>
```

### 行情接口

#### 获取报价
```bash
GET /api/v1/market/quotes/:symbol
Authorization: Bearer <token>
```

响应:
```json
{
  "success": true,
  "quote": {
    "symbol": "600519",
    "price": "1800.00",
    "change": "10.00",
    "change_pct": "0.56",
    "volume": 1000000,
    "timestamp": 1704067200000
  }
}
```

#### 获取K线
```bash
GET /api/v1/market/klines/:symbol?interval=1m&limit=100
Authorization: Bearer <token>
```

#### WebSocket实时行情
```bash
ws://localhost:8083/ws?symbol=600519
```

### 账户接口

#### 获取账户信息
```bash
GET /api/v1/account/:user_id
Authorization: Bearer <token>
```

#### 入金
```bash
POST /api/v1/account/deposit
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "user123",
  "amount": "10000.00"
}
```

#### 出金
```bash
POST /api/v1/account/withdraw
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "user123",
  "amount": "5000.00"
}
```

#### 获取持仓
```bash
GET /api/v1/users/:user_id/positions
Authorization: Bearer <token>
```

### 对账接口

#### 资金对账
```bash
POST /api/v1/reconciliation/funds
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "user123"
}
```

#### 持仓对账
```bash
POST /api/v1/reconciliation/positions
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "user123",
  "symbol": "600519"
}
```

#### 全量对账
```bash
POST /api/v1/reconciliation/full
Authorization: Bearer <token>
```

#### 查询差异
```bash
GET /api/v1/reconciliation/discrepancies?status=OPEN&limit=100
Authorization: Bearer <token>
```

#### 修复差异
```bash
POST /api/v1/reconciliation/discrepancies/:id/fix
Authorization: Bearer <token>
Content-Type: application/json

{
  "fix_type": "ADJUST_BALANCE",
  "notes": "自动修复资金差异",
  "executed_by": "SYSTEM"
}
```

## 一键部署

### 部署命令

```bash
cd deployment

# 默认部署（完整流程）
./deploy.sh deploy

# 仅构建
./deploy.sh build

# 仅启动
./deploy.sh start

# 停止服务
./deploy.sh stop

# 重启服务
./deploy.sh restart

# 查看状态
./deploy.sh status

# 查看日志
./deploy.sh logs api-gateway 50

# 健康检查
./deploy.sh health

# 数据备份
./deploy.sh backup

# 回滚版本
./deploy.sh rollback

# 紧急停止
./deploy.sh emergency-stop

# 清理环境
./deploy.sh cleanup
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| VERSION | v1.0.0 | 版本号 |
| BUILD_PARALLEL | true | 是否并行构建 |
| SKIP_BUILD | false | 跳过构建 |

### 示例

```bash
# 并行构建部署
./deploy.sh deploy

# 串行构建部署
BUILD_PARALLEL=false ./deploy.sh deploy

# 指定版本部署
VERSION=v2.0.0 ./deploy.sh deploy

# 仅查看状态
./deploy.sh status

# 查看订单服务日志
./deploy.sh logs order-service 100
```

## 项目结构

```
stock_trader/
├── backend/
│   ├── services/
│   │   ├── user-service/              # 用户服务 (DDD)
│   │   │   ├── cmd/main.go
│   │   │   ├── config.yaml
│   │   │   └── internal/
│   │   │       ├── domain/
│   │   │       │   ├── entity/
│   │   │       │   ├── service/
│   │   │       │   └── repository/
│   │   │       ├── infrastructure/
│   │   │       │   ├── config/
│   │   │       │   ├── logger/
│   │   │       │   └── security/
│   │   │       └── interfaces/
│   │   │           ├── grpc/
│   │   │           └── http/
│   │   │
│   │   ├── order-service/             # 订单服务 (DDD)
│   │   │   ├── cmd/main.go
│   │   │   ├── config.yaml
│   │   │   └── internal/
│   │   │       ├── domain/
│   │   │       │   ├── entity/
│   │   │       │   ├── service/
│   │   │       │   └── repository/
│   │   │       ├── infrastructure/
│   │   │       │   ├── config/
│   │   │       │   ├── database/
│   │   │       │   ├── logger/
│   │   │       │   └── messaging/
│   │   │       └── interfaces/
│   │   │           ├── grpc/
│   │   │           └── http/
│   │   │
│   │   ├── account-service/           # 账户服务 (DDD)
│   │   │   ├── cmd/main.go
│   │   │   ├── config.yaml
│   │   │   └── internal/
│   │   │       ├── domain/
│   │   │       │   ├── entity/
│   │   │       │   ├── service/
│   │   │       │   └── repository/
│   │   │       ├── infrastructure/
│   │   │       │   ├── config/
│   │   │       │   ├── database/
│   │   │       │   ├── logger/
│   │   │       │   └── messaging/
│   │   │       └── interfaces/
│   │   │           ├── grpc/
│   │   │           └── http/
│   │   │
│   │   ├── market-service/            # 行情服务 (DDD)
│   │   │   ├── cmd/main.go
│   │   │   ├── config.yaml
│   │   │   └── internal/
│   │   │       ├── domain/
│   │   │       │   ├── entity/
│   │   │       │   ├── service/
│   │   │       │   └── repository/
│   │   │       ├── infrastructure/
│   │   │       │   ├── config/
│   │   │       │   ├── logger/
│   │   │       │   └── messaging/
│   │   │       └── interfaces/
│   │   │           ├── grpc/
│   │   │           ├── http/
│   │   │           └── websocket/
│   │   │
│   │   ├── matching-service/          # 撮合服务 (DDD)
│   │   │   ├── cmd/main.go
│   │   │   ├── config.yaml
│   │   │   └── internal/
│   │   │       ├── domain/
│   │   │       │   ├── entity/
│   │   │       │   ├── service/
│   │   │       │   ├── valueobject/
│   │   │       │   └── repository/
│   │   │       ├── infrastructure/
│   │   │       │   ├── config/
│   │   │       │   ├── database/
│   │   │       │   ├── logger/
│   │   │       │   └── messaging/
│   │   │       └── interfaces/
│   │   │           ├── grpc/
│   │   │           ├── http/
│   │   │           └── websocket/
│   │   │
│   │   ├── reconciliation-service/    # 对账服务 (DDD)
│   │   │   ├── cmd/main.go
│   │   │   ├── config.yaml
│   │   │   └── internal/
│   │   │       ├── domain/
│   │   │       │   ├── entity/
│   │   │       │   ├── service/
│   │   │       │   └── repository/
│   │   │       ├── infrastructure/
│   │   │       │   ├── config/
│   │   │       │   └── logger/
│   │   │       └── interfaces/
│   │   │           └── http/
│   │   │
│   │   └── api-gateway/               # API网关 (DDD)
│   │       ├── cmd/main.go
│   │       ├── config.yaml
│   │       └── internal/
│   │           ├── domain/
│   │           │   ├── entity/
│   │           │   └── service/
│   │           ├── infrastructure/
│   │           │   ├── config/
│   │           │   └── logger/
│   │           └── interfaces/
│   │               └── http/
│   │
│   └── Dockerfile Templates
│
├── deployment/                         # 部署配置
│   ├── docker-compose.yml             # Docker Compose编排
│   ├── deploy.sh                      # 一键部署脚本
│   ├── Dockerfile.user-service
│   ├── Dockerfile.order-service
│   ├── Dockerfile.account-service
│   ├── Dockerfile.market-service
│   ├── Dockerfile.matching-service
│   ├── Dockerfile.reconciliation-service
│   └── Dockerfile.api-gateway
│
├── frontend/                           # 前端应用
│   ├── pages/
│   ├── components/
│   └── public/
│
└── README.md                           # 项目文档
```

## 极端情况处理

系统已实现完善的极端情况处理机制：

### 1. 资金安全
- **TCC分布式事务**: Try-Confirm-Cancel模式确保资金安全
- **资金锁机制**: 订单资金锁定，防止重复使用
- **乐观锁**: 版本控制防止并发更新
- **余额检查**: 负数余额自动检测和修复

### 2. 订单处理
- **幂等性设计**: 防止重复订单
- **状态机**: 订单状态流转严格控制
- **超时处理**: 24小时超时自动取消
- **重复订单检测**: 自动取消重复订单

### 3. 并发安全
- **行级锁**: 数据库行级锁防止并发更新
- **Redis原子操作**: 使用Redis原子命令
- **队列削峰**: 订单队列缓冲高并发
- **熔断器**: 防止级联故障

### 4. 系统异常
- **优雅关闭**: 支持优雅停止
- **健康检查**: 自动检测服务状态
- **自动恢复**: 异常服务自动重启
- **日志审计**: 完整操作日志记录

### 5. 数据一致性
- **对账机制**: 定期全量对账
- **差异检测**: 自动检测资金差异
- **自动修复**: 可配置自动修复策略
- **审计追踪**: 完整审计日志

### 6. 风控保护
- **频率限制**: 登录/下单频率限制
- **金额限制**: 单笔订单金额上限
- **价格保护**: 价格偏离检测
- **数量限制**: 单笔订单数量限制

## 监控运维

### 健康检查

```bash
# 检查所有服务健康状态
./deploy.sh health

# 检查单个服务
curl http://localhost:8080/health
curl http://localhost:8081/ready
```

### 日志查看

```bash
# 查看实时日志
./deploy.sh logs api-gateway

# 查看最近100行日志
docker logs --tail=100 stock_trader_api-gateway
```

### 服务状态

```bash
# 查看所有服务状态
./deploy.sh status

# Docker Compose状态
docker-compose ps
```

### 紧急恢复

```bash
# 紧急停止所有服务
./deploy.sh emergency-stop

# 回滚到上一版本
./deploy.sh rollback

# 清理环境（慎用！）
./deploy.sh cleanup
```

### 性能指标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| 订单响应时间 | < 100ms | 从下单到确认 |
| 撮合延迟 | < 10ms | 订单匹配延迟 |
| 并发订单数 | > 10000/s | 系统吞吐量 |
| 可用性 | > 99.99% | 服务可用率 |
| 对账准确率 | 100% | 数据一致性 |

## 许可证

本项目采用MIT许可证 - 详见 [LICENSE](LICENSE) 文件。

## 致谢

感谢所有为这个项目做出贡献的开发者！
