# 证券交易系统重构实施计划

## 一、项目概述

### 1.1 目标
将现有无法运行的项目重构为一个**生产级证券交易系统**，对标同花顺，具备：
- 高并发、高可用、资金安全
- 完整的交易、结算、对账功能
- 技术栈：Kafka、MySQL、Redis、gRPC、Protobuf

### 1.2 现有问题分析
1. Proto 文件未生成 pb.go 代码
2. 配置文件缺失
3. 数据库初始化脚本缺失
4. 多处语法错误和 import 问题
5. 模块路径不一致
6. Dockerfile 构建路径问题

---

## 二、实施计划

### 阶段一：项目结构重构 (预计 2 小时)

#### 1.1 创建新的项目目录结构
```
stock-trading-system/
├── cmd/                          # 应用入口
│   ├── gateway/                  # API 网关
│   ├── account-service/          # 账户服务
│   ├── trading-service/          # 交易服务
│   ├── matching-service/         # 撮合服务
│   ├── settlement-service/       # 结算服务
│   ├── market-service/           # 行情服务
│   └── reconcile-service/        # 对账服务
│
├── internal/                     # 内部代码
│   ├── domain/                   # 领域模型
│   │   ├── account/              # 账户领域
│   │   ├── order/                # 订单领域
│   │   ├── trade/                # 成交领域
│   │   ├── position/             # 持仓领域
│   │   └── matching/             # 撮合领域
│   │
│   ├── service/                  # 业务服务
│   │   ├── account/              # 账户服务
│   │   ├── trading/              # 交易服务
│   │   ├── matching/             # 撮合服务
│   │   ├── settlement/           # 结算服务
│   │   ├── market/               # 行情服务
│   │   └── reconcile/            # 对账服务
│   │
│   └── infrastructure/           # 基础设施
│       ├── database/             # 数据库
│       ├── cache/                # 缓存
│       ├── mq/                   # 消息队列
│       ├── idgen/                # ID 生成器
│       └── config/               # 配置
│
├── api/                          # API 定义
│   └── proto/                    # Proto 文件
│       ├── account.proto
│       ├── trading.proto
│       ├── matching.proto
│       ├── settlement.proto
│       ├── market.proto
│       └── reconcile.proto
│
├── pkg/                          # 公共包
│   ├── errors/                   # 错误定义
│   ├── logger/                   # 日志
│   ├── middleware/               # 中间件
│   └── utils/                    # 工具函数
│
├── scripts/                      # 脚本
│   ├── sql/                      # SQL 脚本
│   │   ├── schema.sql            # 表结构
│   │   └── init_data.sql         # 初始数据
│   └── build.sh                  # 构建脚本
│
├── deployments/                  # 部署配置
│   ├── docker-compose.yml
│   └── Dockerfile
│
├── configs/                      # 配置文件
│   ├── config.yaml               # 默认配置
│   └── config.prod.yaml          # 生产配置
│
├── docs/                         # 文档
│   └── ARCHITECTURE_DESIGN.md    # 架构设计文档
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

#### 1.2 初始化 Go 模块
- 创建统一的 go.mod 文件
- 配置 replace 指令
- 统一依赖版本

---

### 阶段二：数据库设计 (预计 1 小时)

#### 2.1 核心表设计

**账户表 (accounts)**
```sql
CREATE TABLE accounts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(64) NOT NULL UNIQUE COMMENT '用户ID',
    cash_balance DECIMAL(20,4) NOT NULL DEFAULT 0 COMMENT '现金余额',
    frozen_balance DECIMAL(20,4) NOT NULL DEFAULT 0 COMMENT '冻结余额',
    total_assets DECIMAL(20,4) NOT NULL DEFAULT 0 COMMENT '总资产',
    version BIGINT NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='账户表';
```

**持仓表 (positions)**
```sql
CREATE TABLE positions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    symbol VARCHAR(16) NOT NULL COMMENT '证券代码',
    quantity INT NOT NULL DEFAULT 0 COMMENT '持仓数量',
    frozen_quantity INT NOT NULL DEFAULT 0 COMMENT '冻结数量',
    avg_cost DECIMAL(20,4) NOT NULL DEFAULT 0 COMMENT '平均成本',
    version BIGINT NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_symbol (user_id, symbol),
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='持仓表';
```

**订单表 (orders)**
```sql
CREATE TABLE orders (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    order_id VARCHAR(32) NOT NULL UNIQUE COMMENT '订单ID',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    symbol VARCHAR(16) NOT NULL COMMENT '证券代码',
    side ENUM('BUY', 'SELL') NOT NULL COMMENT '买卖方向',
    order_type ENUM('LIMIT', 'MARKET') NOT NULL COMMENT '订单类型',
    price DECIMAL(20,4) NOT NULL COMMENT '委托价格',
    quantity INT NOT NULL COMMENT '委托数量',
    filled_quantity INT NOT NULL DEFAULT 0 COMMENT '成交数量',
    status ENUM('CREATED', 'PENDING', 'PARTIAL', 'FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED') NOT NULL COMMENT '订单状态',
    client_order_id VARCHAR(64) COMMENT '客户端订单ID',
    reject_reason VARCHAR(256) COMMENT '拒绝原因',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_symbol (symbol),
    INDEX idx_status (status),
    INDEX idx_client_order_id (client_order_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';
```

**成交表 (trades)**
```sql
CREATE TABLE trades (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    trade_id VARCHAR(32) NOT NULL UNIQUE COMMENT '成交ID',
    order_id VARCHAR(32) NOT NULL COMMENT '订单ID',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    symbol VARCHAR(16) NOT NULL COMMENT '证券代码',
    side ENUM('BUY', 'SELL') NOT NULL COMMENT '买卖方向',
    price DECIMAL(20,4) NOT NULL COMMENT '成交价格',
    quantity INT NOT NULL COMMENT '成交数量',
    amount DECIMAL(20,4) NOT NULL COMMENT '成交金额',
    fee DECIMAL(20,4) NOT NULL DEFAULT 0 COMMENT '手续费',
    counter_user_id VARCHAR(64) COMMENT '对手方用户ID',
    counter_order_id VARCHAR(32) COMMENT '对手方订单ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_order_id (order_id),
    INDEX idx_user_id (user_id),
    INDEX idx_symbol (symbol),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='成交表';
```

**资金流水表 (fund_flows)**
```sql
CREATE TABLE fund_flows (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    flow_id VARCHAR(32) NOT NULL UNIQUE COMMENT '流水ID',
    transaction_id VARCHAR(64) NOT NULL COMMENT '事务ID',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    amount DECIMAL(20,4) NOT NULL COMMENT '金额',
    flow_type ENUM('DEPOSIT', 'WITHDRAW', 'FREEZE', 'UNFREEZE', 'DEDUCT', 'CREDIT') NOT NULL COMMENT '流水类型',
    balance_before DECIMAL(20,4) NOT NULL COMMENT '变动前余额',
    balance_after DECIMAL(20,4) NOT NULL COMMENT '变动后余额',
    status ENUM('PENDING', 'SUCCESS', 'FAILED') NOT NULL COMMENT '状态',
    remark VARCHAR(256) COMMENT '备注',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_transaction_id (transaction_id),
    INDEX idx_user_id (user_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='资金流水表';
```

**TCC 事务表 (tcc_transactions)**
```sql
CREATE TABLE tcc_transactions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    transaction_id VARCHAR(64) NOT NULL UNIQUE COMMENT '事务ID',
    phase ENUM('TRY', 'CONFIRM', 'CANCEL') NOT NULL COMMENT '阶段',
    status ENUM('PENDING', 'SUCCESS', 'FAILED') NOT NULL COMMENT '状态',
    try_params TEXT COMMENT 'Try 参数',
    retry_count INT NOT NULL DEFAULT 0 COMMENT '重试次数',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='TCC事务表';
```

**对账差异表 (discrepancies)**
```sql
CREATE TABLE discrepancies (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    discrepancy_id VARCHAR(32) NOT NULL UNIQUE COMMENT '差异ID',
    discrepancy_type ENUM('FUND', 'POSITION', 'TRADE') NOT NULL COMMENT '差异类型',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    symbol VARCHAR(16) COMMENT '证券代码',
    expected_value DECIMAL(20,4) NOT NULL COMMENT '预期值',
    actual_value DECIMAL(20,4) NOT NULL COMMENT '实际值',
    difference DECIMAL(20,4) NOT NULL COMMENT '差异值',
    status ENUM('OPEN', 'RESOLVED', 'IGNORED') NOT NULL COMMENT '状态',
    resolved_at DATETIME COMMENT '解决时间',
    resolved_by VARCHAR(64) COMMENT '解决人',
    remark VARCHAR(256) COMMENT '备注',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='对账差异表';
```

---

### 阶段三：Proto 文件定义 (预计 1 小时)

#### 3.1 核心 Proto 文件
- account.proto - 账户服务接口
- trading.proto - 交易服务接口
- matching.proto - 撮合服务接口
- settlement.proto - 结算服务接口
- market.proto - 行情服务接口
- reconcile.proto - 对账服务接口

#### 3.2 生成代码
```bash
# 安装 protoc
# 生成 Go 代码
make proto
```

---

### 阶段四：核心领域模型实现 (预计 3 小时)

#### 4.1 领域模型
- Account (账户聚合根)
- Position (持仓聚合根)
- Order (订单聚合根)
- Trade (成交实体)
- FundFlow (资金流水实体)
- OrderBook (订单簿)

#### 4.2 领域服务
- AccountDomainService (账户领域服务)
- TradingDomainService (交易领域服务)
- MatchingDomainService (撮合领域服务)
- SettlementDomainService (结算领域服务)

#### 4.3 仓储接口与实现
- AccountRepository
- PositionRepository
- OrderRepository
- TradeRepository
- FundFlowRepository

---

### 阶段五：微服务实现 (预计 4 小时)

#### 5.1 账户服务 (Account Service)
- 资金管理 (入金/出金)
- 资金冻结/解冻
- 持仓管理
- 账户查询

#### 5.2 交易服务 (Trading Service)
- 订单创建
- 订单取消
- 订单查询
- 订单校验

#### 5.3 撮合服务 (Matching Service)
- 订单撮合引擎
- 价格优先、时间优先
- 成交生成
- 订单簿管理

#### 5.4 结算服务 (Settlement Service)
- 成交结算
- 资金划转
- 持仓更新
- 手续费计算

#### 5.5 行情服务 (Market Service)
- 实时行情
- K 线数据
- 盘口数据
- WebSocket 推送

#### 5.6 对账服务 (Reconcile Service)
- 资金对账
- 持仓对账
- 成交对账
- 差异修复

#### 5.7 API 网关 (API Gateway)
- 路由转发
- 认证鉴权
- 限流熔断
- 协议转换

---

### 阶段六：配置与部署 (预计 1 小时)

#### 6.1 配置文件
- config.yaml (默认配置)
- config.prod.yaml (生产配置)

#### 6.2 Docker 配置
- Dockerfile (多阶段构建)
- docker-compose.yml (服务编排)

#### 6.3 构建脚本
- Makefile
- build.sh

---

### 阶段七：测试与验证 (预计 1 小时)

#### 7.1 单元测试
- 领域模型测试
- 业务逻辑测试

#### 7.2 集成测试
- 服务间通信测试
- 事务一致性测试

#### 7.3 压力测试
- 订单创建性能测试
- 撮合引擎性能测试

---

## 三、实施顺序

```
┌─────────────────────────────────────────────────────────────────┐
│                        实施顺序                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Step 1: 项目结构重构                                          │
│      ↓                                                          │
│   Step 2: 数据库表创建                                          │
│      ↓                                                          │
│   Step 3: Proto 文件定义 + 代码生成                             │
│      ↓                                                          │
│   Step 4: 公共包实现 (errors, logger, config, idgen)            │
│      ↓                                                          │
│   Step 5: 基础设施实现 (database, cache, mq)                    │
│      ↓                                                          │
│   Step 6: 领域模型实现                                          │
│      ↓                                                          │
│   Step 7: 仓储层实现                                            │
│      ↓                                                          │
│   Step 8: 领域服务实现                                          │
│      ↓                                                          │
│   Step 9: gRPC 服务实现                                         │
│      ↓                                                          │
│   Step 10: HTTP 服务实现                                        │
│      ↓                                                          │
│   Step 11: API 网关实现                                         │
│      ↓                                                          │
│   Step 12: 配置文件 + 部署脚本                                  │
│      ↓                                                          │
│   Step 13: 测试验证                                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 四、关键技术决策

### 4.1 订单号生成
- 使用 Redis INCR 生成序列号
- 格式: 日期(8位) + 服务ID(2位) + 用户ID后4位 + 序列号(6位) + 校验码(2位)
- 总长度: 22位

### 4.2 分布式事务
- 资金冻结: TCC 模式
- 成交结算: Saga 模式
- 对账修复: 最大努力通知

### 4.3 幂等性保证
- 订单: client_order_id + Redis SETNX
- 资金操作: transaction_id + 数据库唯一索引
- 消息消费: message_id + Redis SETNX

### 4.4 并发控制
- 分布式锁: Redis SETNX
- 乐观锁: version 字段
- 悲观锁: SELECT ... FOR UPDATE

### 4.5 资金安全
- 应用层校验 + 数据库层保护
- 资金流水完整记录
- 实时对账 + 定时对账

---

## 五、预期成果

### 5.1 功能完整性
- ✅ 完整的交易流程 (下单 → 撮合 → 结算)
- ✅ 资金管理 (入金/出金/冻结/解冻)
- ✅ 持仓管理 (买入/卖出/查询)
- ✅ 对账机制 (资金/持仓/成交)
- ✅ 行情服务 (实时行情/K线/盘口)

### 5.2 非功能特性
- ✅ 高并发: 支持万级 QPS
- ✅ 高可用: 99.99% 可用性
- ✅ 资金安全: 多重保障机制
- ✅ 数据一致性: 强一致性保证

### 5.3 代码质量
- ✅ 清晰的 DDD 架构
- ✅ 完整的错误处理
- ✅ 详细的日志记录
- ✅ 单元测试覆盖

---

## 六、风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|----------|
| 时间不足 | 中 | 优先实现核心功能，非核心功能后续迭代 |
| 技术难点 | 高 | 提前调研，准备备选方案 |
| 数据一致性 | 高 | 严格测试，完善对账机制 |
| 性能问题 | 中 | 压力测试，优化瓶颈 |

---

**计划版本**: v1.0  
**创建时间**: 2024-03-15  
**预计总工时**: 13 小时
