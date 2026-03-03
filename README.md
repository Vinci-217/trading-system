# 证券交易系统 (Stock Trading System)

一个完整的、高性能的生产级证券交易系统，采用DDD领域驱动设计和微服务架构，支持股票买卖订单管理、实时行情撮合、资金持仓管理、对账风控等核心功能。

## 项目概述

本系统是一个完整的证券交易平台，具有以下特点：

- **DDD架构**: 清晰的四层架构设计，领域模型与业务逻辑分离
- **高性能**: 采用Go语言开发，支持高并发、低延迟交易撮合
- **高可用**: 微服务架构，支持水平扩展和故障转移
- **资金安全**: TCC分布式事务确保资金安全
- **完整风控**: 多层次风控体系，防止异常交易
- **对账系统**: 自动对账检测和修复资金差异

## 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.21+ | 高性能后端开发 |
| 数据库 | MySQL 8.0 | 主数据存储 |
| 缓存 | Redis 7.0 | 分布式缓存、分布式锁 |
| 消息队列 | Kafka | 事件驱动、异步通信 |
| RPC | gRPC + Protobuf | 服务间通信 |
| HTTP框架 | Gin | RESTful API |

## 快速开始

### 环境要求

- Go 1.21+
- MySQL 8.0+
- Redis 7.0+
- Kafka 3.0+ (可选)

### 安装依赖

```bash
git clone https://github.com/Vinci-217/trading-system.git
cd trading-system
go mod download
```

### 初始化数据库

```bash
mysql -u root -p < scripts/sql/schema.sql
```

### 启动服务

```bash
# 启动交易服务
go run ./cmd/trading-service

# 启动账户服务
go run ./cmd/account-service

# 启动行情服务
go run ./cmd/market-service

# 启动撮合服务
go run ./cmd/matching-service

# 启动结算服务
go run ./cmd/settlement-service

# 启动对账服务
go run ./cmd/reconcile-service

# 启动网关服务
go run ./cmd/gateway-service
```

## 服务架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           客户端 (Browser/Mobile)                        │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Gateway Service (:8080)                               │
│              统一入口 │ 路由转发 │ 认证授权 │ 限流熔断                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
        ┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
        │  Account Service  │ │ Trading Service   │ │  Market Service   │
        │   (:8081/50051)   │ │  (:8082/50052)    │ │  (:8083/50053)    │
        │                   │ │                   │ │                   │
        │ 账户余额管理       │ │ 订单管理          │ │ 实时行情报价       │
        │ 资金冻结/解冻      │ │ 撮合交易          │ │ K线数据           │
        │ 持仓管理           │ │ 订单查询/撤单     │ │ WebSocket推送     │
        └───────────────────┘ └───────────────────┘ └───────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
        ┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
        │ Matching Service  │ │ Settlement Service│ │ Reconcile Service │
        │   (:8084/50054)   │ │  (:8085/50055)    │ │  (:8086/50056)    │
        │                   │ │                   │ │                   │
        │ 订单撮合引擎       │ │ 成交清算          │ │ 资金对账          │
        │ 盘口深度          │ │ 资金划转          │ │ 持仓对账          │
        │ 买卖价差          │ │ 持仓更新          │ │ 差异修复          │
        └───────────────────┘ └───────────────────┘ └───────────────────┘
                                    │
                                    ▼
        ┌─────────────────────────────────────────────────────────────────┐
        │                        中间件服务                                │
        │  ┌──────────┐  ┌──────────┐  ┌──────────┐                      │
        │  │  MySQL   │  │  Redis   │  │  Kafka   │                      │
        │  │ (:3306)  │  │ (:6379)  │  │ (:9092)  │                      │
        │  └──────────┘  └──────────┘  └──────────┘                      │
        └─────────────────────────────────────────────────────────────────┘
```

## 服务端口

| 服务 | HTTP端口 | gRPC端口 | 功能 |
|------|----------|----------|------|
| gateway-service | 8080 | 50050 | API网关 |
| account-service | 8081 | 50051 | 账户管理 |
| trading-service | 8082 | 50052 | 交易服务 |
| market-service | 8083 | 50053 | 行情服务 |
| matching-service | 8084 | 50054 | 撮合服务 |
| settlement-service | 8085 | 50055 | 结算服务 |
| reconcile-service | 8086 | 50056 | 对账服务 |

## API文档

### 账户接口

```bash
# 查询账户
GET /api/v1/accounts/:user_id

# 入金
POST /api/v1/accounts/deposit
{"user_id":"user001","amount":"10000.00"}

# 出金
POST /api/v1/accounts/withdraw
{"user_id":"user001","amount":"1000.00"}

# 查询持仓
GET /api/v1/accounts/:user_id/positions
```

### 订单接口

```bash
# 创建订单
POST /api/v1/orders
{
  "user_id": "user001",
  "symbol": "600000",
  "side": 1,          // 1=买入, 2=卖出
  "order_type": 1,    // 1=限价单, 2=市价单
  "price": "10.50",
  "quantity": 100
}

# 查询订单
GET /api/v1/orders/:order_id?user_id=user001

# 撤销订单
POST /api/v1/orders/:order_id/cancel?user_id=user001

# 查询用户订单列表
GET /api/v1/orders?user_id=user001&symbol=600000&status=2
```

### 行情接口

```bash
# 获取行情报价
GET /api/v1/quotes/:symbol

# 获取所有行情
GET /api/v1/quotes

# 获取盘口深度
GET /api/v1/depth/:symbol?level=5

# WebSocket实时行情
ws://localhost:8083/api/v1/ws/quotes/:symbol
```

### 撮合接口

```bash
# 获取订单簿
GET /api/v1/orderbook/:symbol

# 获取盘口快照
GET /api/v1/snapshot/:symbol?depth=10

# 获取最新成交价
GET /api/v1/lastprice/:symbol

# 获取买卖价差
GET /api/v1/spread/:symbol
```

### 对账接口

```bash
# 账户对账
POST /api/v1/reconcile/account/:user_id

# 持仓对账
POST /api/v1/reconcile/position/:user_id/:symbol

# 全量持仓对账
POST /api/v1/reconcile/positions

# 日终对账
POST /api/v1/reconcile/daily/:date

# 查询差异记录
GET /api/v1/discrepancies?user_id=user001

# 解决差异
POST /api/v1/discrepancies/:id/resolve
```

## 核心设计

### 订单状态机

```
CREATED(1) ──→ PENDING(2) ──→ PARTIAL(3) ──→ FILLED(4)
                  │               │
                  └───────────────┴──→ CANCELLED(5)
```

### 订单ID设计

22位结构化订单ID：`YYYYMMDD + 服务ID(2位) + 用户ID后4位 + 序列号(5位) + 校验位`

示例：`2026030301r001000005F5`

### 幂等性设计

- Redis SETNX 分布式锁
- 数据库唯一索引 (order_id, client_order_id)
- 请求ID追踪

### TCC分布式事务

```
Try:   冻结资金 → 检查余额 → 锁定资源
Confirm: 扣减资金 → 创建订单 → 确认交易
Cancel: 解冻资金 → 释放资源 → 回滚状态
```

### 资金安全

- 乐观锁 (version字段)
- 余额校验 `cash_balance - frozen_balance >= amount`
- 冻结/解冻原子操作
- 资金流水完整追踪

## 性能测试

### 测试环境

- CPU: 4核
- 内存: 8GB
- MySQL: 本地实例
- Redis: 本地实例

### 测试结果

| 接口 | QPS | 平均响应时间 | 并发数 |
|------|-----|-------------|--------|
| 账户查询 | 642 req/s | 77ms | 50 |
| 订单创建 | 107 req/s | 93ms | 10 |
| 健康检查 | 3000+ req/s | 30ms | 100 |

## 项目结构

```
trading-system/
├── cmd/                          # 服务入口
│   ├── gateway-service/          # API网关
│   ├── account-service/          # 账户服务
│   ├── trading-service/          # 交易服务
│   ├── market-service/           # 行情服务
│   ├── matching-service/         # 撮合服务
│   ├── settlement-service/       # 结算服务
│   └── reconcile-service/        # 对账服务
├── internal/
│   ├── domain/                   # 领域层
│   │   ├── account/              # 账户领域
│   │   ├── order/                # 订单领域
│   │   ├── trade/                # 成交领域
│   │   └── matching/             # 撮合引擎
│   ├── service/                  # 应用服务层
│   │   ├── account/              # 账户服务
│   │   ├── trading/              # 交易服务
│   │   ├── market/               # 行情服务
│   │   ├── matching/             # 撮合服务
│   │   ├── settlement/           # 结算服务
│   │   └── reconcile/            # 对账服务
│   └── infrastructure/           # 基础设施层
│       ├── config/               # 配置管理
│       ├── database/             # 数据库连接
│       ├── cache/                # Redis缓存
│       ├── mq/                   # Kafka消息队列
│       └── idgen/                # ID生成器
├── pkg/                          # 公共包
│   ├── errors/                   # 错误定义
│   └── logger/                   # 日志组件
├── configs/                      # 配置文件
├── scripts/                      # 脚本
│   └── sql/                      # 数据库脚本
├── docs/                         # 文档
├── api/                          # Proto定义
├── Makefile                      # 构建脚本
└── README.md                     # 项目文档
```

## 极端情况处理

### 资金安全
- TCC分布式事务确保资金操作原子性
- 乐观锁防止并发更新冲突
- 余额不足自动回滚

### 订单处理
- 幂等性设计防止重复下单
- 状态机严格控制订单流转
- 分布式锁防止并发操作

### 系统异常
- 服务健康检查
- 优雅关闭支持
- 完整日志审计

### 数据一致性
- 定期对账检测差异
- 自动/手动修复机制
- 资金流水完整追踪

## 监控运维

```bash
# 健康检查
curl http://localhost:8082/health

# 查看服务状态
curl http://localhost:8082/ready
```

## 许可证

MIT License

## 致谢

感谢所有为这个项目做出贡献的开发者！
