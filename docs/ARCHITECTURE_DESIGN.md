# 证券交易系统 - 完整架构设计文档

## 目录

1. [系统概述](#1-系统概述)
2. [整体架构设计](#2-整体架构设计)
3. [交易状态机设计](#3-交易状态机设计)
4. [幂等性设计](#4-幂等性设计)
5. [分布式事务一致性设计](#5-分布式事务一致性设计)
6. [订单号设计](#6-订单号设计)
7. [数据库高可用设计](#7-数据库高可用设计)
8. [资金安全设计](#8-资金安全设计)
9. [原子性与一致性设计](#9-原子性与一致性设计)
10. [并发控制设计](#10-并发控制设计)
11. [消息队列设计](#11-消息队列设计)
12. [缓存设计](#12-缓存设计)

---

## 1. 系统概述

### 1.1 系统定位

本系统是一个对标同花顺的生产级证券交易系统，核心特性：

- **高并发**: 支持每秒万级订单处理
- **高可用**: 99.99% 可用性保障
- **资金安全**: 多重保障机制，资金零差错
- **数据一致性**: 强一致性保障

### 1.2 核心功能模块

```
┌─────────────────────────────────────────────────────────────────┐
│                        证券交易系统                               │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  行情服务    │  │  交易服务    │  │  账户服务    │            │
│  │  Market     │  │  Trading    │  │  Account    │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  撮合服务    │  │  结算服务    │  │  对账服务    │            │
│  │  Matching   │  │  Settlement │  │  Reconcile  │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. 整体架构设计

### 2.1 微服务架构

```
                                    ┌─────────────────┐
                                    │    客户端        │
                                    └────────┬────────┘
                                             │
                                             ▼
┌────────────────────────────────────────────────────────────────────┐
│                         API Gateway (网关层)                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐             │
│  │ 认证鉴权  │ │ 限流熔断  │ │ 路由转发  │ │ 协议转换  │             │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘             │
└────────────────────────────────────────────────────────────────────┘
                                             │
                    ┌────────────────────────┼────────────────────────┐
                    │                        │                        │
                    ▼                        ▼                        ▼
         ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
         │   Account Svc    │    │   Trading Svc    │    │   Market Svc     │
         │   账户服务        │    │   交易服务        │    │   行情服务        │
         │                  │    │                  │    │                  │
         │ • 资金管理       │    │ • 订单管理       │    │ • 实时行情       │
         │ • 持仓管理       │    │ • 订单校验       │    │ • K线数据        │
         │ • 资金冻结       │    │ • 订单路由       │    │ • 盘口数据       │
         └────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘
                  │                       │                       │
                  └───────────────────────┼───────────────────────┘
                                          │
                                          ▼
                              ┌───────────────────────┐
                              │      Kafka Cluster    │
                              │      消息总线          │
                              └───────────┬───────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
                    ▼                     ▼                     ▼
         ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
         │  Matching Svc    │  │  Settlement Svc  │  │  Reconcile Svc   │
         │  撮合服务         │  │  结算服务         │  │  对账服务         │
         │                  │  │                  │  │                  │
         │ • 订单撮合       │  │ • 成交结算       │  │ • 资金对账       │
         │ • 价格确定       │  │ • 资金划转       │  │ • 持仓对账       │
         │ • 成交生成       │  │ • 持仓更新       │  │ • 差异修复       │
         └──────────────────┘  └──────────────────┘  └──────────────────┘
```

### 2.2 服务端口分配

| 服务 | gRPC端口 | HTTP端口 | 说明 |
|------|----------|----------|------|
| API Gateway | - | 8080 | 统一入口 |
| Account Service | 50051 | 8081 | 账户管理 |
| Trading Service | 50052 | 8082 | 交易管理 |
| Market Service | 50053 | 8083 | 行情服务 |
| Matching Service | 50054 | 8084 | 撮合服务 |
| Settlement Service | 50055 | 8085 | 结算服务 |
| Reconciliation Service | 50056 | 8086 | 对账服务 |

---

## 3. 交易状态机设计

### 3.1 订单状态机

```
                                    ┌─────────────┐
                                    │   CREATED   │
                                    │   已创建     │
                                    └──────┬──────┘
                                           │
                          ┌────────────────┼────────────────┐
                          │                │                │
                          ▼                ▼                ▼
                   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
                   │  SUBMITTED  │  │  REJECTED   │  │   EXPIRED   │
                   │   已提交     │  │   已拒绝     │  │   已过期     │
                   └──────┬──────┘  └─────────────┘  └─────────────┘
                          │
                          ▼
                   ┌─────────────┐
                   │   PENDING   │
                   │   待撮合     │
                   └──────┬──────┘
                          │
            ┌─────────────┼─────────────┐
            │             │             │
            ▼             ▼             ▼
     ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
     │  PARTIAL    │ │   FILLED    │ │  CANCELLED  │
     │   部分成交   │ │   完全成交   │ │   已撤单     │
     └──────┬──────┘ └─────────────┘ └─────────────┘
            │
            ├──────────────────────┐
            │                      │
            ▼                      ▼
     ┌─────────────┐        ┌─────────────┐
     │   FILLED    │        │  CANCELLED  │
     │   完全成交   │        │   已撤单     │
     └─────────────┘        └─────────────┘
```

### 3.2 状态转换规则

```go
// 订单状态定义
type OrderStatus string

const (
    OrderStatusCreated   OrderStatus = "CREATED"    // 已创建，未提交
    OrderStatusSubmitted OrderStatus = "SUBMITTED"  // 已提交，等待校验
    OrderStatusPending   OrderStatus = "PENDING"    // 待撮合
    OrderStatusPartial   OrderStatus = "PARTIAL"    // 部分成交
    OrderStatusFilled    OrderStatus = "FILLED"     // 完全成交
    OrderStatusCancelled OrderStatus = "CANCELLED"  // 已撤单
    OrderStatusRejected  OrderStatus = "REJECTED"   // 已拒绝
    OrderStatusExpired   OrderStatus = "EXPIRED"    // 已过期
)

// 状态转换矩阵
var OrderStatusTransitions = map[OrderStatus][]OrderStatus{
    OrderStatusCreated:   {OrderStatusSubmitted, OrderStatusRejected, OrderStatusExpired},
    OrderStatusSubmitted: {OrderStatusPending, OrderStatusRejected},
    OrderStatusPending:   {OrderStatusPartial, OrderStatusFilled, OrderStatusCancelled},
    OrderStatusPartial:   {OrderStatusFilled, OrderStatusCancelled},
    OrderStatusFilled:    {}, // 终态
    OrderStatusCancelled: {}, // 终态
    OrderStatusRejected:  {}, // 终态
    OrderStatusExpired:   {}, // 终态
}

// 状态转换校验
func (o *Order) CanTransitionTo(target OrderStatus) bool {
    allowed, exists := OrderStatusTransitions[o.Status]
    if !exists {
        return false
    }
    for _, s := range allowed {
        if s == target {
            return true
        }
    }
    return false
}
```

### 3.3 资金冻结状态机

```
     ┌─────────────┐
     │    NONE     │
     │   无冻结     │
     └──────┬──────┘
            │ 冻结请求
            ▼
     ┌─────────────┐
     │   LOCKING   │
     │   冻结中     │
     └──────┬──────┘
            │
     ┌──────┴──────┐
     │             │
     ▼             ▼
┌─────────────┐ ┌─────────────┐
│   LOCKED    │ │  LOCK_FAIL  │
│   已冻结     │ │   冻结失败   │
└──────┬──────┘ └─────────────┘
       │
       ├─────────────────────┐
       │                     │
       ▼                     ▼
┌─────────────┐       ┌─────────────┐
│  CONFIRMED  │       │   RELEASED  │
│   已确认     │       │   已释放     │
└─────────────┘       └─────────────┘
```

### 3.4 成交结算状态机

```
     ┌─────────────┐
     │   PENDING   │
     │   待结算     │
     └──────┬──────┘
            │
            ▼
     ┌─────────────┐
     │  SETTLING   │
     │   结算中     │
     └──────┬──────┘
            │
     ┌──────┴──────┐
     │             │
     ▼             ▼
┌─────────────┐ ┌─────────────┐
│   SUCCESS   │ │   FAILED    │
│   结算成功   │ │   结算失败   │
└─────────────┘ └──────┬──────┘
                       │
                       ▼
                ┌─────────────┐
                │   RETRY     │
                │   重试中     │
                └─────────────┘
```

---

## 4. 幂等性设计

### 4.1 幂等性原则

**核心思想**: 同一操作执行多次与执行一次的效果相同。

### 4.2 订单幂等性设计

```go
// 订单幂等键设计
type OrderIdempotency struct {
    UserID        string    `json:"user_id"`         // 用户ID
    ClientOrderID string    `json:"client_order_id"` // 客户端订单ID (UUID)
    CreatedAt     time.Time `json:"created_at"`      // 创建时间
}

// 幂等性校验流程
func (s *TradingService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // 1. 生成幂等键
    idempotencyKey := fmt.Sprintf("order:%s:%s", req.UserID, req.ClientOrderID)
    
    // 2. Redis SETNX 实现分布式锁 + 幂等校验
    ok, err := s.redis.SetNX(ctx, idempotencyKey, "processing", 30*time.Second).Result()
    if err != nil {
        return nil, err
    }
    
    if !ok {
        // 3. 键已存在，检查是否是重复请求
        existingOrder, err := s.orderRepo.GetByClientOrderID(ctx, req.UserID, req.ClientOrderID)
        if err == nil {
            // 返回已存在的订单
            return existingOrder, nil
        }
        // 正在处理中，返回错误
        return nil, ErrOrderProcessing
    }
    
    // 4. 执行订单创建逻辑
    order, err := s.doCreateOrder(ctx, req)
    if err != nil {
        s.redis.Del(ctx, idempotencyKey)
        return nil, err
    }
    
    // 5. 更新幂等键状态
    s.redis.Set(ctx, idempotencyKey, order.OrderID, 24*time.Hour)
    
    return order, nil
}
```

### 4.3 资金操作幂等性设计

```go
// 资金流水幂等性
type FundFlow struct {
    ID           string          `json:"id"`            // 流水ID
    TransactionID string         `json:"transaction_id"`// 事务ID (幂等键)
    UserID       string          `json:"user_id"`
    Amount       decimal.Decimal `json:"amount"`
    Type         string          `json:"type"`          // DEPOSIT/WITHDRAW/FREEZE/UNFREEZE
    Status       string          `json:"status"`
    CreatedAt    time.Time       `json:"created_at"`
}

// 资金操作幂等性保证
func (s *AccountService) FreezeFunds(ctx context.Context, req *FreezeFundsRequest) error {
    // 使用事务ID作为幂等键
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 1. 检查事务ID是否已存在
    var exists int
    err = tx.QueryRowContext(ctx, 
        "SELECT 1 FROM fund_flows WHERE transaction_id = ? FOR UPDATE", 
        req.TransactionID).Scan(&exists)
    
    if err != sql.ErrNoRows {
        // 事务ID已存在，幂等返回
        if err == nil {
            return nil
        }
        return err
    }
    
    // 2. 执行资金冻结
    result, err := tx.ExecContext(ctx,
        "UPDATE accounts SET frozen_balance = frozen_balance + ?, version = version + 1 WHERE user_id = ? AND (cash_balance - frozen_balance) >= ?",
        req.Amount, req.UserID, req.Amount)
    
    if err != nil {
        return err
    }
    
    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrInsufficientBalance
    }
    
    // 3. 记录资金流水
    _, err = tx.ExecContext(ctx,
        "INSERT INTO fund_flows (id, transaction_id, user_id, amount, type, status) VALUES (?, ?, ?, ?, 'FREEZE', 'SUCCESS')",
        uuid.New().String(), req.TransactionID, req.UserID, req.Amount)
    
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### 4.4 消息消费幂等性

```go
// Kafka 消息幂等消费
type MessageConsumer struct {
    redis  *redis.Client
    db     *sql.DB
}

func (c *MessageConsumer) ConsumeTradeMessage(msg *TradeMessage) error {
    ctx := context.Background()
    
    // 1. 使用消息ID作为幂等键
    idempotencyKey := fmt.Sprintf("msg:trade:%s", msg.MessageID)
    
    // 2. Redis SETNX 检查
    ok, err := c.redis.SetNX(ctx, idempotencyKey, "1", 7*24*time.Hour).Result()
    if err != nil {
        return err
    }
    
    if !ok {
        // 消息已处理，直接确认
        return nil
    }
    
    // 3. 执行业务逻辑 (使用数据库事务保证原子性)
    err = c.processTrade(ctx, msg)
    if err != nil {
        // 处理失败，删除幂等键，允许重试
        c.redis.Del(ctx, idempotencyKey)
        return err
    }
    
    return nil
}
```

---

## 5. 分布式事务一致性设计

### 5.1 事务模式选择

| 场景 | 事务模式 | 说明 |
|------|----------|------|
| 订单创建 | 本地事务 + 事件驱动 | 单服务内完成 |
| 资金冻结 | TCC | 跨服务，需要补偿 |
| 成交结算 | Saga | 长事务，需要补偿 |
| 对账修复 | 最大努力通知 | 允许一定延迟 |

### 5.2 TCC 事务设计 (资金冻结)

```
┌─────────────────────────────────────────────────────────────────┐
│                        TCC 事务流程                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌─────────┐      ┌─────────┐      ┌─────────┐               │
│   │  Try    │ ──── │ Confirm │ ──── │  Done   │               │
│   │  尝试   │      │  确认   │      │  完成   │               │
│   └────┬────┘      └────┬────┘      └─────────┘               │
│        │                │                                      │
│        │ 失败           │ 失败                                 │
│        ▼                ▼                                      │
│   ┌─────────┐      ┌─────────┐                                │
│   │ Cancel  │      │ Cancel  │                                │
│   │  取消   │      │  取消   │                                │
│   └─────────┘      └─────────┘                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

```go
// TCC 事务协调器
type TCCCoordinator struct {
    db    *sql.DB
    redis *redis.Client
}

// TCC 事务记录
type TCCRecord struct {
    ID            string    `json:"id"`
    TransactionID string    `json:"transaction_id"`
    Phase         string    `json:"phase"`         // TRY / CONFIRM / CANCEL
    Status        string    `json:"status"`        // PENDING / SUCCESS / FAILED
    TryParams     string    `json:"try_params"`    // Try 阶段参数
    ConfirmParams string    `json:"confirm_params"`// Confirm 阶段参数
    CancelParams  string    `json:"cancel_params"` // Cancel 阶段参数
    RetryCount    int       `json:"retry_count"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}

// Try 阶段：冻结资金
func (c *TCCCoordinator) Try(ctx context.Context, req *FreezeRequest) error {
    tx, err := c.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 1. 创建 TCC 记录
    record := &TCCRecord{
        ID:            uuid.New().String(),
        TransactionID: req.TransactionID,
        Phase:         "TRY",
        Status:        "PENDING",
        TryParams:     toJSON(req),
    }
    
    _, err = tx.ExecContext(ctx,
        "INSERT INTO tcc_records (id, transaction_id, phase, status, try_params) VALUES (?, ?, 'TRY', 'PENDING', ?)",
        record.ID, record.TransactionID, record.TryParams)
    if err != nil {
        return err
    }
    
    // 2. 冻结资金 (使用乐观锁)
    result, err := tx.ExecContext(ctx,
        "UPDATE accounts SET frozen_balance = frozen_balance + ?, version = version + 1 WHERE user_id = ? AND (cash_balance - frozen_balance) >= ? AND version = ?",
        req.Amount, req.UserID, req.Amount, req.Version)
    if err != nil {
        return err
    }
    
    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrInsufficientBalance
    }
    
    // 3. 更新 TCC 记录状态
    _, err = tx.ExecContext(ctx,
        "UPDATE tcc_records SET status = 'SUCCESS', updated_at = NOW() WHERE id = ?",
        record.ID)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}

// Confirm 阶段：确认扣款
func (c *TCCCoordinator) Confirm(ctx context.Context, transactionID string) error {
    tx, err := c.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 1. 查询 TCC 记录
    var record TCCRecord
    err = tx.QueryRowContext(ctx,
        "SELECT id, transaction_id, phase, status FROM tcc_records WHERE transaction_id = ? FOR UPDATE",
        transactionID).Scan(&record.ID, &record.TransactionID, &record.Phase, &record.Status)
    if err != nil {
        return err
    }
    
    // 2. 幂等性检查
    if record.Status == "CONFIRMED" {
        return nil
    }
    
    // 3. 确认扣款 (冻结资金 -> 扣除)
    req := parseFreezeRequest(record.TryParams)
    result, err := tx.ExecContext(ctx,
        "UPDATE accounts SET cash_balance = cash_balance - ?, frozen_balance = frozen_balance - ?, version = version + 1 WHERE user_id = ?",
        req.Amount, req.Amount, req.UserID)
    if err != nil {
        return err
    }
    
    // 4. 更新 TCC 记录
    _, err = tx.ExecContext(ctx,
        "UPDATE tcc_records SET phase = 'CONFIRM', status = 'SUCCESS', updated_at = NOW() WHERE id = ?",
        record.ID)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}

// Cancel 阶段：释放冻结资金
func (c *TCCCoordinator) Cancel(ctx context.Context, transactionID string) error {
    tx, err := c.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 1. 查询 TCC 记录
    var record TCCRecord
    err = tx.QueryRowContext(ctx,
        "SELECT id, transaction_id, phase, status, try_params FROM tcc_records WHERE transaction_id = ? FOR UPDATE",
        transactionID).Scan(&record.ID, &record.TransactionID, &record.Phase, &record.Status, &record.TryParams)
    if err != nil {
        return err
    }
    
    // 2. 幂等性检查
    if record.Status == "CANCELLED" {
        return nil
    }
    
    // 3. 释放冻结资金
    req := parseFreezeRequest(record.TryParams)
    result, err := tx.ExecContext(ctx,
        "UPDATE accounts SET frozen_balance = frozen_balance - ?, version = version + 1 WHERE user_id = ?",
        req.Amount, req.UserID)
    if err != nil {
        return err
    }
    
    // 4. 更新 TCC 记录
    _, err = tx.ExecContext(ctx,
        "UPDATE tcc_records SET phase = 'CANCEL', status = 'SUCCESS', updated_at = NOW() WHERE id = ?",
        record.ID)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### 5.3 Saga 事务设计 (成交结算)

```
┌─────────────────────────────────────────────────────────────────┐
│                      Saga 事务流程 (成交结算)                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐ │
│  │ Step 1   │───▶│ Step 2   │───▶│ Step 3   │───▶│ Step 4   │ │
│  │ 买方扣款 │    │ 卖方加款 │    │ 买方加仓 │    │ 卖方减仓 │ │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘ │
│       │               │               │               │        │
│       │ 失败          │ 失败          │ 失败          │ 失败   │
│       ▼               ▼               ▼               ▼        │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐ │
│  │ Comp 1   │◀───│ Comp 2   │◀───│ Comp 3   │◀───│ Comp 4   │ │
│  │ 买方退款 │    │ 卖方扣款 │    │ 买方减仓 │    │ 卖方加仓 │ │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘ │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

```go
// Saga 事务定义
type SagaDefinition struct {
    Steps []SagaStep
}

type SagaStep struct {
    Name            string
    Execute         func(ctx context.Context, data interface{}) error
    Compensate     func(ctx context.Context, data interface{}) error
}

// 成交结算 Saga
var SettlementSaga = SagaDefinition{
    Steps: []SagaStep{
        {
            Name: "DeductBuyerFunds",
            Execute: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return deductFunds(ctx, trade.BuyerID, trade.Amount)
            },
            Compensate: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return refundFunds(ctx, trade.BuyerID, trade.Amount)
            },
        },
        {
            Name: "CreditSellerFunds",
            Execute: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return creditFunds(ctx, trade.SellerID, trade.Amount)
            },
            Compensate: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return deductFunds(ctx, trade.SellerID, trade.Amount)
            },
        },
        {
            Name: "AddBuyerPosition",
            Execute: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return addPosition(ctx, trade.BuyerID, trade.Symbol, trade.Quantity)
            },
            Compensate: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return reducePosition(ctx, trade.BuyerID, trade.Symbol, trade.Quantity)
            },
        },
        {
            Name: "ReduceSellerPosition",
            Execute: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return reducePosition(ctx, trade.SellerID, trade.Symbol, trade.Quantity)
            },
            Compensate: func(ctx context.Context, data interface{}) error {
                trade := data.(*Trade)
                return addPosition(ctx, trade.SellerID, trade.Symbol, trade.Quantity)
            },
        },
    },
}

// Saga 执行器
type SagaExecutor struct {
    db       *sql.DB
    redis    *redis.Client
    producer *kafka.Producer
}

func (e *SagaExecutor) Execute(ctx context.Context, saga SagaDefinition, data interface{}, sagaID string) error {
    // 记录 Saga 状态
    tx, _ := e.db.BeginTx(ctx, nil)
    
    // 执行各步骤
    completedSteps := make([]int, 0)
    
    for i, step := range saga.Steps {
        // 记录步骤开始
        e.recordStep(ctx, sagaID, i, step.Name, "STARTED")
        
        err := step.Execute(ctx, data)
        if err != nil {
            // 执行补偿
            e.recordStep(ctx, sagaID, i, step.Name, "FAILED")
            e.compensate(ctx, saga, completedSteps, data, sagaID)
            return err
        }
        
        // 记录步骤完成
        e.recordStep(ctx, sagaID, i, step.Name, "COMPLETED")
        completedSteps = append(completedSteps, i)
    }
    
    tx.Commit()
    return nil
}

func (e *SagaExecutor) compensate(ctx context.Context, saga SagaDefinition, completedSteps []int, data interface{}, sagaID string) {
    // 逆序执行补偿
    for i := len(completedSteps) - 1; i >= 0; i-- {
        stepIdx := completedSteps[i]
        step := saga.Steps[stepIdx]
        
        e.recordStep(ctx, sagaID, stepIdx, step.Name, "COMPENSATING")
        
        err := step.Compensate(ctx, data)
        if err != nil {
            // 补偿失败，记录并告警
            e.recordStep(ctx, sagaID, stepIdx, step.Name, "COMPENSATE_FAILED")
            // 发送告警
            e.alertCompensationFailure(sagaID, stepIdx, step.Name)
            continue
        }
        
        e.recordStep(ctx, sagaID, stepIdx, step.Name, "COMPENSATED")
    }
}
```

---

## 6. 订单号设计

### 6.1 订单号设计原则

1. **全局唯一**: 分布式环境下不重复
2. **有序性**: 按时间递增，便于索引和查询
3. **可读性**: 包含业务含义
4. **安全性**: 不暴露业务量信息

### 6.2 订单号结构设计

```
┌─────────────────────────────────────────────────────────────┐
│                        订单号结构                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────────────┐    │
│   │ 日期  │ │服务ID│ │用户ID│ │序列号│ │    校验码    │    │
│   │8位   │ │2位  │ │4位  │ │6位  │ │     2位      │    │
│   └──────┘ └──────┘ └──────┘ └──────┘ └──────────────┘    │
│                                                             │
│   示例: 20240315 01 0001 000001 AB                         │
│         │     │  │    │      │                             │
│         │     │  │    │      └─ 校验码 (CRC16)             │
│         │     │  │    └──────── 序列号 (Redis生成)          │
│         │     │  └───────────── 用户ID后4位                 │
│         │     └──────────────── 服务ID (01=交易服务)        │
│         └────────────────────── 日期 (YYYYMMDD)            │
│                                                             │
│   总长度: 22位                                              │
└─────────────────────────────────────────────────────────────┘
```

### 6.3 订单号生成器实现

```go
// 订单号生成器
type OrderIDGenerator struct {
    redis    *redis.Client
    serviceID string  // 服务ID，如 "01"
}

func NewOrderIDGenerator(redis *redis.Client, serviceID string) *OrderIDGenerator {
    return &OrderIDGenerator{
        redis:     redis,
        serviceID: serviceID,
    }
}

func (g *OrderIDGenerator) Generate(ctx context.Context, userID string) (string, error) {
    // 1. 日期部分 (8位)
    datePart := time.Now().Format("20060102")
    
    // 2. 服务ID (2位)
    servicePart := g.serviceID
    
    // 3. 用户ID后4位 (4位)
    userPart := fmt.Sprintf("%04s", userID[len(userID)-4:])
    
    // 4. 序列号 (6位) - 使用 Redis INCR 保证原子性
    seqKey := fmt.Sprintf("order:seq:%s", datePart)
    seq, err := g.redis.Incr(ctx, seqKey).Result()
    if err != nil {
        return "", err
    }
    
    // 设置过期时间 (2天)
    g.redis.Expire(ctx, seqKey, 48*time.Hour)
    
    seqPart := fmt.Sprintf("%06d", seq%1000000)
    
    // 5. 组合原始订单号
    rawID := datePart + servicePart + userPart + seqPart
    
    // 6. 计算校验码 (2位)
    checkSum := g.calculateChecksum(rawID)
    
    // 7. 最终订单号
    orderID := rawID + checkSum
    
    return orderID, nil
}

func (g *OrderIDGenerator) calculateChecksum(id string) string {
    // 使用 CRC16 算法
    crc := crc16.ChecksumIEEE([]byte(id))
    return fmt.Sprintf("%02X", crc%256)
}

func (g *OrderIDGenerator) Validate(orderID string) bool {
    if len(orderID) != 22 {
        return false
    }
    
    rawID := orderID[:20]
    checkSum := orderID[20:]
    
    expected := g.calculateChecksum(rawID)
    return checkSum == expected
}
```

### 6.4 其他ID生成策略

```go
// 成交号生成 (Snowflake 算法)
type TradeIDGenerator struct {
    node *snowflake.Node
}

func NewTradeIDGenerator(nodeID int64) (*TradeIDGenerator, error) {
    node, err := snowflake.NewNode(nodeID)
    if err != nil {
        return nil, err
    }
    return &TradeIDGenerator{node: node}, nil
}

func (g *TradeIDGenerator) Generate() string {
    return g.node.Generate().String()
}

// 资金流水号生成
type FlowIDGenerator struct {
    redis *redis.Client
}

func (g *FlowIDGenerator) Generate(ctx context.Context, flowType string) (string, error) {
    date := time.Now().Format("20060102")
    key := fmt.Sprintf("flow:seq:%s:%s", flowType, date)
    
    seq, err := g.redis.Incr(ctx, key).Result()
    if err != nil {
        return "", err
    }
    
    g.redis.Expire(ctx, key, 48*time.Hour)
    
    return fmt.Sprintf("%s%s%08d", flowType, date, seq), nil
}
```

---

## 7. 数据库高可用设计

### 7.1 MySQL 高可用架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    MySQL 高可用架构                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│                      ┌─────────────┐                           │
│                      │   MHA/MGR   │                           │
│                      │  故障检测    │                           │
│                      └──────┬──────┘                           │
│                             │                                   │
│         ┌───────────────────┼───────────────────┐              │
│         │                   │                   │              │
│         ▼                   ▼                   ▼              │
│   ┌───────────┐       ┌───────────┐       ┌───────────┐      │
│   │  Master   │ ────▶ │  Slave1   │       │  Slave2   │      │
│   │  写节点    │  复制  │  读节点    │       │  读节点    │      │
│   │  (主)     │       │  (从)     │       │  (从)     │      │
│   └─────┬─────┘       └───────────┘       └───────────┘      │
│         │                                                       │
│         │ 自动切换                                               │
│         ▼                                                       │
│   ┌───────────┐                                                │
│   │  Slave1   │                                                │
│   │  新主节点  │                                                │
│   └───────────┘                                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 7.2 读写分离配置

```go
// 数据库连接池配置
type DBConfig struct {
    MasterDSN      string
    SlaveDSNs      []string
    MaxOpenConns   int
    MaxIdleConns   int
    ConnMaxLifetime time.Duration
}

// 读写分离路由器
type DBRouter struct {
    master *sql.DB
    slaves []*sql.DB
    lb     *loadbalancer.RoundRobin
}

func NewDBRouter(cfg *DBConfig) (*DBRouter, error) {
    // 连接主库
    master, err := sql.Open("mysql", cfg.MasterDSN)
    if err != nil {
        return nil, err
    }
    master.SetMaxOpenConns(cfg.MaxOpenConns)
    master.SetMaxIdleConns(cfg.MaxIdleConns)
    master.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    
    // 连接从库
    slaves := make([]*sql.DB, 0, len(cfg.SlaveDSNs))
    for _, dsn := range cfg.SlaveDSNs {
        slave, err := sql.Open("mysql", dsn)
        if err != nil {
            continue
        }
        slave.SetMaxOpenConns(cfg.MaxOpenConns / 2)
        slave.SetMaxIdleConns(cfg.MaxIdleConns / 2)
        slaves = append(slaves, slave)
    }
    
    return &DBRouter{
        master: master,
        slaves: slaves,
        lb:     loadbalancer.NewRoundRobin(len(slaves)),
    }, nil
}

// 写操作使用主库
func (r *DBRouter) Master() *sql.DB {
    return r.master
}

// 读操作使用从库 (轮询)
func (r *DBRouter) Slave() *sql.DB {
    if len(r.slaves) == 0 {
        return r.master
    }
    idx := r.lb.Next()
    return r.slaves[idx]
}
```

### 7.3 数据库分库分表策略

```
┌─────────────────────────────────────────────────────────────────┐
│                     分库分表策略                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  分库键: user_id (用户ID)                                       │
│  分表键: created_at (创建时间，按月分表)                         │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    orders 表                            │   │
│  ├─────────────────────────────────────────────────────────┤   │
│  │  db0.orders_202401  (user_id % 4 == 0)                 │   │
│  │  db0.orders_202402  (user_id % 4 == 0)                 │   │
│  │  db1.orders_202401  (user_id % 4 == 1)                 │   │
│  │  db1.orders_202402  (user_id % 4 == 1)                 │   │
│  │  db2.orders_202401  (user_id % 4 == 2)                 │   │
│  │  db2.orders_202402  (user_id % 4 == 2)                 │   │
│  │  db3.orders_202401  (user_id % 4 == 3)                 │   │
│  │  db3.orders_202402  (user_id % 4 == 3)                 │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  路由算法:                                                     │
│  • 库: db_index = user_id % 4                                  │
│  • 表: table_suffix = YYYYMM (按月)                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 7.4 数据备份策略

```yaml
# 备份策略配置
backup:
  # 全量备份
  full:
    schedule: "0 2 * * 0"  # 每周日凌晨2点
    retention: 30          # 保留30天
    
  # 增量备份
  incremental:
    schedule: "0 2 * * 1-6"  # 周一到周六凌晨2点
    retention: 7             # 保留7天
    
  # binlog 备份
  binlog:
    enabled: true
    retention: 7
    
  # 异地备份
  remote:
    enabled: true
    endpoint: "s3://backup-bucket/mysql/"
```

---

## 8. 资金安全设计

### 8.1 资金安全原则

1. **永不丢失**: 任何情况下资金数据不丢失
2. **永不超扣**: 余额永远不为负
3. **可追溯**: 所有资金变动有完整流水
4. **可恢复**: 支持数据恢复和纠错

### 8.2 多重保障机制

```
┌─────────────────────────────────────────────────────────────────┐
│                      资金安全多重保障                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   第1层: 应用层校验                                             │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │ • 余额预校验 (可用余额 >= 操作金额)                       │  │
│   │ • 金额合法性校验 (金额 > 0)                               │  │
│   │ • 业务规则校验 (单笔限额、日累计限额)                     │  │
│   └─────────────────────────────────────────────────────────┘  │
│                                                                 │
│   第2层: 数据库层保护                                           │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │ • WHERE 条件保护 (WHERE balance >= amount)               │  │
│   │ • 乐观锁保护 (version 字段)                               │  │
│   │ • 行级锁保护 (SELECT ... FOR UPDATE)                     │  │
│   │ • 触发器保护 (余额非负约束)                               │  │
│   └─────────────────────────────────────────────────────────┘  │
│                                                                 │
│   第3层: 流水记录                                               │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │ • 双向流水 (借/贷)                                        │  │
│   │ • 原子性写入 (事务)                                       │  │
│   │ • 流水号唯一 (幂等)                                       │  │
│   └─────────────────────────────────────────────────────────┘  │
│                                                                 │
│   第4层: 对账机制                                               │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │ • 实时对账 (关键操作后)                                   │  │
│   │ • 定时对账 (每小时/每天)                                  │  │
│   │ • 差异告警 (差异 > 阈值)                                  │  │
│   │ • 自动修复 (小额差异)                                     │  │
│   └─────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 8.3 资金操作实现

```go
// 资金服务 - 安全操作
type FundService struct {
    db    *sql.DB
    redis *redis.Client
}

// 资金冻结 (安全实现)
func (s *FundService) Freeze(ctx context.Context, req *FreezeRequest) error {
    // 1. 参数校验
    if req.Amount.LessThanOrEqual(decimal.Zero) {
        return ErrInvalidAmount
    }
    
    // 2. 获取分布式锁 (防止并发操作同一账户)
    lockKey := fmt.Sprintf("lock:account:%s", req.UserID)
    lock, err := s.acquireDistributedLock(ctx, lockKey, 10*time.Second)
    if err != nil {
        return err
    }
    defer s.releaseDistributedLock(ctx, lockKey)
    
    // 3. 开启事务
    tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelRepeatableRead,
    })
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 4. 查询当前余额 (加行锁)
    var account Account
    err = tx.QueryRowContext(ctx,
        "SELECT user_id, cash_balance, frozen_balance, version FROM accounts WHERE user_id = ? FOR UPDATE",
        req.UserID).Scan(&account.UserID, &account.CashBalance, &account.FrozenBalance, &account.Version)
    if err != nil {
        return err
    }
    
    // 5. 应用层余额校验
    available := account.CashBalance.Sub(account.FrozenBalance)
    if available.LessThan(req.Amount) {
        return ErrInsufficientBalance
    }
    
    // 6. 幂等性检查
    var exists int
    err = tx.QueryRowContext(ctx,
        "SELECT 1 FROM fund_flows WHERE transaction_id = ?",
        req.TransactionID).Scan(&exists)
    if err == nil {
        return nil // 幂等返回
    }
    
    // 7. 更新账户 (乐观锁 + WHERE 条件保护)
    result, err := tx.ExecContext(ctx,
        `UPDATE accounts 
         SET frozen_balance = frozen_balance + ?, 
             version = version + 1,
             updated_at = NOW()
         WHERE user_id = ? 
           AND (cash_balance - frozen_balance) >= ?
           AND version = ?`,
        req.Amount, req.UserID, req.Amount, account.Version)
    if err != nil {
        return err
    }
    
    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrConcurrentUpdate
    }
    
    // 8. 记录资金流水
    flowID := uuid.New().String()
    _, err = tx.ExecContext(ctx,
        `INSERT INTO fund_flows 
         (id, transaction_id, user_id, amount, flow_type, balance_before, balance_after, status, created_at)
         VALUES (?, ?, ?, ?, 'FREEZE', ?, ?, 'SUCCESS', NOW())`,
        flowID, req.TransactionID, req.UserID, req.Amount, 
        account.CashBalance, account.CashBalance)
    if err != nil {
        return err
    }
    
    // 9. 提交事务
    return tx.Commit()
}

// 资金解冻
func (s *FundService) Unfreeze(ctx context.Context, req *UnfreezeRequest) error {
    // 类似冻结流程，反向操作
    // ...
}

// 资金扣款 (确认冻结)
func (s *FundService) Deduct(ctx context.Context, req *DeductRequest) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 幂等性检查
    var exists int
    err = tx.QueryRowContext(ctx,
        "SELECT 1 FROM fund_flows WHERE transaction_id = ? AND flow_type = 'DEDUCT'",
        req.TransactionID).Scan(&exists)
    if err == nil {
        return nil
    }
    
    // 原子性扣款: 冻结余额减少 + 现金余额减少
    result, err := tx.ExecContext(ctx,
        `UPDATE accounts 
         SET cash_balance = cash_balance - ?,
             frozen_balance = frozen_balance - ?,
             version = version + 1,
             updated_at = NOW()
         WHERE user_id = ?
           AND frozen_balance >= ?
           AND version = ?`,
        req.Amount, req.Amount, req.UserID, req.Amount, req.Version)
    if err != nil {
        return err
    }
    
    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrInsufficientFrozenBalance
    }
    
    // 记录流水
    _, err = tx.ExecContext(ctx,
        `INSERT INTO fund_flows (id, transaction_id, user_id, amount, flow_type, status, created_at)
         VALUES (?, ?, ?, ?, 'DEDUCT', 'SUCCESS', NOW())`,
        uuid.New().String(), req.TransactionID, req.UserID, req.Amount)
    
    return tx.Commit()
}
```

### 8.4 资金对账机制

```go
// 资金对账服务
type ReconciliationService struct {
    db       *sql.DB
    redis    *redis.Client
    producer *kafka.Producer
}

// 实时对账 (关键操作后触发)
func (s *ReconciliationService) RealtimeReconcile(ctx context.Context, userID string) error {
    // 1. 计算账户应有余额
    var account Account
    err := s.db.QueryRowContext(ctx,
        "SELECT user_id, cash_balance, frozen_balance FROM accounts WHERE user_id = ?",
        userID).Scan(&account.UserID, &account.CashBalance, &account.FrozenBalance)
    if err != nil {
        return err
    }
    
    // 2. 计算流水汇总
    var flowSum struct {
        TotalDeposit   decimal.Decimal
        TotalWithdraw  decimal.Decimal
        TotalFreeze    decimal.Decimal
        TotalUnfreeze  decimal.Decimal
        TotalDeduct    decimal.Decimal
        TotalCredit    decimal.Decimal
    }
    
    err = s.db.QueryRowContext(ctx,
        `SELECT 
            COALESCE(SUM(CASE WHEN flow_type = 'DEPOSIT' THEN amount ELSE 0 END), 0) as total_deposit,
            COALESCE(SUM(CASE WHEN flow_type = 'WITHDRAW' THEN amount ELSE 0 END), 0) as total_withdraw,
            COALESCE(SUM(CASE WHEN flow_type = 'FREEZE' THEN amount ELSE 0 END), 0) as total_freeze,
            COALESCE(SUM(CASE WHEN flow_type = 'UNFREEZE' THEN amount ELSE 0 END), 0) as total_unfreeze,
            COALESCE(SUM(CASE WHEN flow_type = 'DEDUCT' THEN amount ELSE 0 END), 0) as total_deduct,
            COALESCE(SUM(CASE WHEN flow_type = 'CREDIT' THEN amount ELSE 0 END), 0) as total_credit
         FROM fund_flows 
         WHERE user_id = ? AND status = 'SUCCESS'`,
        userID).Scan(&flowSum.TotalDeposit, &flowSum.TotalWithdraw, 
                     &flowSum.TotalFreeze, &flowSum.TotalUnfreeze,
                     &flowSum.TotalDeduct, &flowSum.TotalCredit)
    if err != nil {
        return err
    }
    
    // 3. 计算预期余额
    expectedCashBalance := flowSum.TotalDeposit.
        Sub(flowSum.TotalWithdraw).
        Sub(flowSum.TotalDeduct).
        Add(flowSum.TotalCredit)
    
    expectedFrozenBalance := flowSum.TotalFreeze.
        Sub(flowSum.TotalUnfreeze).
        Sub(flowSum.TotalDeduct)
    
    // 4. 比对差异
    cashDiscrepancy := account.CashBalance.Sub(expectedCashBalance)
    frozenDiscrepancy := account.FrozenBalance.Sub(expectedFrozenBalance)
    
    threshold := decimal.NewFromFloat(0.01) // 1分钱阈值
    
    if cashDiscrepancy.Abs().GreaterThan(threshold) || 
       frozenDiscrepancy.Abs().GreaterThan(threshold) {
        // 5. 记录差异
        s.recordDiscrepancy(ctx, &Discrepancy{
            UserID:            userID,
            Type:              "FUND",
            ExpectedCash:      expectedCashBalance,
            ActualCash:        account.CashBalance,
            CashDiscrepancy:   cashDiscrepancy,
            ExpectedFrozen:    expectedFrozenBalance,
            ActualFrozen:      account.FrozenBalance,
            FrozenDiscrepancy: frozenDiscrepancy,
            Status:            "OPEN",
        })
        
        // 6. 发送告警
        s.sendAlert(ctx, "资金差异告警", userID, cashDiscrepancy, frozenDiscrepancy)
    }
    
    return nil
}

// 定时全量对账
func (s *ReconciliationService) FullReconciliation(ctx context.Context) (*ReconciliationReport, error) {
    report := &ReconciliationReport{
        StartTime: time.Now(),
    }
    
    // 获取所有账户
    rows, err := s.db.QueryContext(ctx, "SELECT user_id FROM accounts")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var userID string
        if err := rows.Scan(&userID); err != nil {
            continue
        }
        
        if err := s.RealtimeReconcile(ctx, userID); err != nil {
            report.FailedAccounts++
        } else {
            report.CheckedAccounts++
        }
    }
    
    report.EndTime = time.Now()
    return report, nil
}
```

---

## 9. 原子性与一致性设计

### 9.1 数据库事务隔离级别

```go
// 事务隔离级别选择
var TransactionIsolationLevels = map[string]sql.IsolationLevel{
    // 订单创建: 读已提交 (避免脏读，允许不可重复读)
    "order_create": sql.LevelReadCommitted,
    
    // 资金操作: 可重复读 (保证一致性)
    "fund_operation": sql.LevelRepeatableRead,
    
    // 持仓更新: 可重复读
    "position_update": sql.LevelRepeatableRead,
    
    // 对账操作: 串行化 (保证强一致性)
    "reconciliation": sql.LevelSerializable,
}
```

### 9.2 原子性保证

```go
// 原子性操作示例: 订单创建 + 资金冻结
func (s *TradingService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // 1. 生成订单ID
    orderID, _ := s.orderIDGen.Generate(ctx, req.UserID)
    
    // 2. 开启事务
    tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelRepeatableRead,
    })
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    // 3. 创建订单记录
    order := &Order{
        OrderID:   orderID,
        UserID:    req.UserID,
        Symbol:    req.Symbol,
        Side:      req.Side,
        Price:     req.Price,
        Quantity:  req.Quantity,
        Status:    OrderStatusCreated,
        CreatedAt: time.Now(),
    }
    
    _, err = tx.ExecContext(ctx,
        `INSERT INTO orders (order_id, user_id, symbol, side, price, quantity, status, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        order.OrderID, order.UserID, order.Symbol, order.Side, 
        order.Price, order.Quantity, order.Status, order.CreatedAt)
    if err != nil {
        return nil, err
    }
    
    // 4. 冻结资金 (买入订单)
    if req.Side == "BUY" {
        freezeAmount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))
        
        result, err := tx.ExecContext(ctx,
            `UPDATE accounts 
             SET frozen_balance = frozen_balance + ?, version = version + 1
             WHERE user_id = ? AND (cash_balance - frozen_balance) >= ?`,
            freezeAmount, req.UserID, freezeAmount)
        if err != nil {
            return nil, err
        }
        
        affected, _ := result.RowsAffected()
        if affected == 0 {
            return nil, ErrInsufficientBalance
        }
        
        // 记录资金流水
        _, err = tx.ExecContext(ctx,
            `INSERT INTO fund_flows (id, transaction_id, user_id, amount, flow_type, status)
             VALUES (?, ?, ?, ?, 'FREEZE', 'SUCCESS')`,
            uuid.New().String(), orderID, req.UserID, freezeAmount)
        if err != nil {
            return nil, err
        }
    }
    
    // 5. 提交事务
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    // 6. 发送订单事件 (异步)
    s.producer.SendMessage(ctx, "order.created", order)
    
    return order, nil
}
```

### 9.3 最终一致性保证

```go
// 最终一致性检查器
type ConsistencyChecker struct {
    db    *sql.DB
    redis *redis.Client
}

// 检查订单-资金一致性
func (c *ConsistencyChecker) CheckOrderFundConsistency(ctx context.Context, orderID string) error {
    // 1. 获取订单信息
    var order Order
    err := c.db.QueryRowContext(ctx,
        "SELECT order_id, user_id, side, price, quantity, filled_quantity, status FROM orders WHERE order_id = ?",
        orderID).Scan(&order.OrderID, &order.UserID, &order.Side, &order.Price, 
                      &order.Quantity, &order.FilledQuantity, &order.Status)
    if err != nil {
        return err
    }
    
    // 2. 获取资金流水
    rows, err := c.db.QueryContext(ctx,
        "SELECT flow_type, amount FROM fund_flows WHERE transaction_id = ?",
        orderID)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    var totalFrozen, totalDeduct decimal.Decimal
    for rows.Next() {
        var flowType string
        var amount decimal.Decimal
        rows.Scan(&flowType, &amount)
        
        switch flowType {
        case "FREEZE":
            totalFrozen = totalFrozen.Add(amount)
        case "DEDUCT":
            totalDeduct = totalDeduct.Add(amount)
        case "UNFREEZE":
            totalFrozen = totalFrozen.Sub(amount)
        }
    }
    
    // 3. 计算预期冻结金额
    expectedFrozen := order.Price.Mul(decimal.NewFromInt(int64(order.Quantity - order.FilledQuantity)))
    
    // 4. 比对
    if !totalFrozen.Sub(totalDeduct).Equals(expectedFrozen) {
        // 记录不一致
        c.recordInconsistency(ctx, "ORDER_FUND", orderID, 
            totalFrozen.Sub(totalDeduct), expectedFrozen)
    }
    
    return nil
}
```

---

## 10. 并发控制设计

### 10.1 分布式锁

```go
// Redis 分布式锁实现
type DistributedLock struct {
    redis   *redis.Client
    key     string
    value   string
    ttl     time.Duration
}

func (r *RedisClient) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*DistributedLock, error) {
    value := uuid.New().String()
    
    // SET key value NX EX ttl
    ok, err := r.redis.SetNX(ctx, key, value, ttl).Result()
    if err != nil {
        return nil, err
    }
    
    if !ok {
        return nil, ErrLockAcquireFailed
    }
    
    return &DistributedLock{
        redis: r.redis,
        key:   key,
        value: value,
        ttl:   ttl,
    }, nil
}

func (l *DistributedLock) Release(ctx context.Context) error {
    // Lua 脚本保证原子性
    script := `
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        else
            return 0
        end
    `
    
    _, err := l.redis.Eval(ctx, script, []string{l.key}, l.value).Result()
    return err
}

// 使用示例
func (s *TradingService) CancelOrder(ctx context.Context, orderID string) error {
    // 获取订单锁
    lockKey := fmt.Sprintf("lock:order:%s", orderID)
    lock, err := s.redis.AcquireLock(ctx, lockKey, 10*time.Second)
    if err != nil {
        return ErrOrderLocked
    }
    defer lock.Release(ctx)
    
    // 执行撤单逻辑
    return s.doCancelOrder(ctx, orderID)
}
```

### 10.2 乐观锁

```go
// 乐观锁实现 (version 字段)
type OptimisticLock struct {
    db *sql.DB
}

func (o *OptimisticLock) UpdateWithVersion(ctx context.Context, table string, 
    id string, updates map[string]interface{}, version int64) error {
    
    // 构建 SET 子句
    setClauses := make([]string, 0, len(updates)+1)
    args := make([]interface{}, 0, len(updates)+2)
    
    for k, v := range updates {
        setClauses = append(setClauses, fmt.Sprintf("%s = ?", k))
        args = append(args, v)
    }
    setClauses = append(setClauses, "version = version + 1")
    
    // 添加 WHERE 条件参数
    args = append(args, id, version)
    
    query := fmt.Sprintf(
        "UPDATE %s SET %s WHERE id = ? AND version = ?",
        table, strings.Join(setClauses, ", "))
    
    result, err := o.db.ExecContext(ctx, query, args...)
    if err != nil {
        return err
    }
    
    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrOptimisticLockConflict
    }
    
    return nil
}
```

### 10.3 悲观锁

```go
// 悲观锁实现 (SELECT ... FOR UPDATE)
func (s *AccountService) Transfer(ctx context.Context, fromUserID, toUserID string, amount decimal.Decimal) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 按固定顺序加锁，避免死锁
    userIDs := []string{fromUserID, toUserID}
    sort.Strings(userIDs)
    
    // 锁定两个账户
    for _, userID := range userIDs {
        _, err := tx.ExecContext(ctx,
            "SELECT 1 FROM accounts WHERE user_id = ? FOR UPDATE",
            userID)
        if err != nil {
            return err
        }
    }
    
    // 执行转账
    result, err := tx.ExecContext(ctx,
        "UPDATE accounts SET cash_balance = cash_balance - ? WHERE user_id = ? AND cash_balance >= ?",
        amount, fromUserID, amount)
    if err != nil {
        return err
    }
    
    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrInsufficientBalance
    }
    
    _, err = tx.ExecContext(ctx,
        "UPDATE accounts SET cash_balance = cash_balance + ? WHERE user_id = ?",
        amount, toUserID)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

---

## 11. 消息队列设计

### 11.1 Kafka Topic 设计

```
┌─────────────────────────────────────────────────────────────────┐
│                      Kafka Topic 设计                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Topic 名称                    │ 分区数 │ 说明                  │
│  ─────────────────────────────────────────────────────────────  │
│  order.created                 │ 12    │ 订单创建事件          │
│  order.cancelled               │ 12    │ 订单取消事件          │
│  order.matched                 │ 12    │ 订单成交事件          │
│  trade.settlement              │ 12    │ 成交结算事件          │
│  account.fund.changed          │ 6     │ 资金变动事件          │
│  account.position.changed      │ 6     │ 持仓变动事件          │
│  market.quote.realtime         │ 3     │ 实时行情推送          │
│  reconciliation.task           │ 3     │ 对账任务              │
│  reconciliation.discrepancy    │ 3     │ 对账差异告警          │
│                                                                 │
│  分区策略:                                                     │
│  • 订单相关: order_id 或 user_id 作为 key                      │
│  • 账户相关: user_id 作为 key                                  │
│  • 行情相关: symbol 作为 key                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 11.2 消息格式设计

```go
// 消息基础结构
type Message struct {
    MessageID   string      `json:"message_id"`    // 消息唯一ID
    MessageType string      `json:"message_type"`  // 消息类型
    Version     string      `json:"version"`       // 消息版本
    Timestamp   int64       `json:"timestamp"`     // 消息时间戳
    Source      string      `json:"source"`        // 消息来源服务
    Payload     interface{} `json:"payload"`       // 消息内容
}

// 订单创建消息
type OrderCreatedMessage struct {
    Message
    Payload struct {
        OrderID    string          `json:"order_id"`
        UserID     string          `json:"user_id"`
        Symbol     string          `json:"symbol"`
        Side       string          `json:"side"`
        Price      decimal.Decimal `json:"price"`
        Quantity   int             `json:"quantity"`
        OrderType  string          `json:"order_type"`
        CreatedAt  int64           `json:"created_at"`
    } `json:"payload"`
}

// 成交消息
type TradeExecutedMessage struct {
    Message
    Payload struct {
        TradeID    string          `json:"trade_id"`
        OrderID    string          `json:"order_id"`
        Symbol     string          `json:"symbol"`
        BuyerID    string          `json:"buyer_id"`
        SellerID   string          `json:"seller_id"`
        Price      decimal.Decimal `json:"price"`
        Quantity   int             `json:"quantity"`
        Amount     decimal.Decimal `json:"amount"`
        ExecutedAt int64           `json:"executed_at"`
    } `json:"payload"`
}
```

### 11.3 消息可靠性保证

```go
// 生产者配置
type ProducerConfig struct {
    // 确认机制: all (所有副本确认)
    Acks string `yaml:"acks"`
    
    // 重试次数
    Retries int `yaml:"retries"`
    
    // 批量大小
    BatchSize int `yaml:"batch_size"`
    
    // 压缩算法
    Compression string `yaml:"compression"`
    
    // 幂等生产者
    EnableIdempotence bool `yaml:"enable_idempotence"`
}

// 消费者配置
type ConsumerConfig struct {
    // 消费组ID
    GroupID string `yaml:"group_id"`
    
    // 自动提交偏移量
    EnableAutoCommit bool `yaml:"enable_auto_commit"`
    
    // 自动提交间隔
    AutoCommitInterval time.Duration `yaml:"auto_commit_interval"`
    
    // 起始偏移量
    AutoOffsetReset string `yaml:"auto_offset_reset"` // earliest/latest
    
    // 最大轮询记录数
    MaxPollRecords int `yaml:"max_poll_records"`
}

// 消息发送 (可靠发送)
func (p *Producer) SendMessage(ctx context.Context, topic string, key string, msg interface{}) error {
    msgBytes, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    // 添加消息ID
    messageID := uuid.New().String()
    
    // 发送消息
    record := &kafka.ProducerMessage{
        Topic: topic,
        Key:   sarama.StringEncoder(key),
        Value: sarama.ByteEncoder(msgBytes),
        Headers: []sarama.RecordHeader{
            {Key: []byte("message_id"), Value: []byte(messageID)},
            {Key: []byte("timestamp"), Value: []byte(time.Now().Format(time.RFC3339))},
        },
    }
    
    // 同步发送，等待确认
    partition, offset, err := p.producer.SendMessage(record)
    if err != nil {
        return err
    }
    
    log.Printf("消息发送成功: topic=%s, partition=%d, offset=%d, message_id=%s",
        topic, partition, offset, messageID)
    
    return nil
}

// 消息消费 (可靠消费)
func (c *Consumer) Consume(ctx context.Context, handler func(msg *Message) error) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case err := <-c.consumer.Errors():
            log.Printf("消费错误: %v", err)
        case msg := <-c.consumer.Messages():
            // 解析消息
            var message Message
            if err := json.Unmarshal(msg.Value, &message); err != nil {
                log.Printf("消息解析失败: %v", err)
                continue
            }
            
            // 处理消息 (幂等)
            if err := handler(&message); err != nil {
                log.Printf("消息处理失败: %v, message_id=%s", err, message.MessageID)
                // 发送到死信队列
                c.sendToDeadLetterQueue(msg, err)
                continue
            }
            
            // 手动提交偏移量
            c.consumer.MarkOffset(msg, "")
        }
    }
}
```

---

## 12. 缓存设计

### 12.1 缓存策略

```
┌─────────────────────────────────────────────────────────────────┐
│                        缓存策略设计                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  数据类型           │ 缓存策略   │ 过期时间  │ 说明              │
│  ─────────────────────────────────────────────────────────────  │
│  用户账户信息       │ Cache-Aside│ 5分钟    │ 读多写少          │
│  用户持仓信息       │ Write-Through│ 1分钟  │ 实时性要求高      │
│  实时行情           │ Write-Behind│ 1秒    │ 高频更新          │
│  K线数据           │ Cache-Aside│ 1分钟   │ 历史数据          │
│  订单簿            │ 本地缓存   │ 实时     │ 撮合引擎内存      │
│  分布式锁          │ Redis SETNX│ 30秒    │ 自动过期          │
│  限流计数器        │ Redis INCR │ 滑动窗口  │ 频率控制          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 12.2 缓存实现

```go
// 缓存管理器
type CacheManager struct {
    redis *redis.Client
    local *freecache.Cache
}

// 多级缓存获取
func (c *CacheManager) Get(ctx context.Context, key string, dest interface{}, 
    loader func() (interface{}, error)) error {
    
    // 1. 尝试本地缓存
    if val, err := c.local.Get([]byte(key)); err == nil {
        return json.Unmarshal(val, dest)
    }
    
    // 2. 尝试 Redis 缓存
    val, err := c.redis.Get(ctx, key).Result()
    if err == nil {
        // 回填本地缓存
        c.local.Set([]byte(key), []byte(val), 60)
        return json.Unmarshal([]byte(val), dest)
    }
    
    // 3. 从数据库加载
    data, err := loader()
    if err != nil {
        return err
    }
    
    // 4. 写入缓存
    bytes, _ := json.Marshal(data)
    c.redis.Set(ctx, key, string(bytes), 5*time.Minute)
    c.local.Set([]byte(key), bytes, 60)
    
    // 5. 返回数据
    bytes, _ = json.Marshal(data)
    return json.Unmarshal(bytes, dest)
}

// 缓存穿透保护
func (c *CacheManager) GetWithBloomFilter(ctx context.Context, key string, 
    dest interface{}, loader func() (interface{}, error)) error {
    
    // 1. 布隆过滤器检查
    if !c.bloomFilter.Test([]byte(key)) {
        return ErrNotFound
    }
    
    // 2. 正常缓存获取流程
    return c.Get(ctx, key, dest, loader)
}

// 缓存雪崩保护
func (c *CacheManager) SetWithRandomExpire(ctx context.Context, key string, 
    value interface{}, baseExpire time.Duration) error {
    
    // 添加随机过期时间 (避免同时过期)
    randomExpire := baseExpire + time.Duration(rand.Intn(60))*time.Second
    
    bytes, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    return c.redis.Set(ctx, key, string(bytes), randomExpire).Err()
}
```

---

## 附录

### A. 关键配置参数

```yaml
# 系统配置
system:
  name: "stock-trading-system"
  version: "1.0.0"
  env: "production"

# 数据库配置
database:
  master:
    host: "mysql-master"
    port: 3306
    database: "stock_trading"
    max_open_conns: 100
    max_idle_conns: 20
    conn_max_lifetime: 300s
    
  slaves:
    - host: "mysql-slave-1"
      port: 3306
    - host: "mysql-slave-2"
      port: 3306

# Redis 配置
redis:
  master: "redis-master:6379"
  slaves:
    - "redis-slave-1:6379"
    - "redis-slave-2:6379"
  pool_size: 100
  min_idle_conns: 20

# Kafka 配置
kafka:
  brokers:
    - "kafka-1:9092"
    - "kafka-2:9092"
    - "kafka-3:9092"
  producer:
    acks: "all"
    retries: 3
    enable_idempotence: true
  consumer:
    group_id: "stock-trading-group"
    auto_offset_reset: "earliest"
    enable_auto_commit: false

# 业务配置
trading:
  # 订单配置
  order:
    max_quantity: 1000000      # 单笔最大数量
    min_quantity: 1            # 单笔最小数量
    max_amount: 10000000       # 单笔最大金额
    price_deviation: 0.1       # 价格偏离阈值 (10%)
    expire_time: 86400         # 订单过期时间 (秒)
    
  # 限流配置
  rate_limit:
    order_create: 100          # 每秒最大下单数
    order_cancel: 50           # 每秒最大撤单数
    
  # 对账配置
  reconciliation:
    discrepancy_threshold: 0.01 # 差异阈值 (元)
    auto_fix_enabled: true      # 自动修复开关
    auto_fix_max_amount: 100    # 自动修复最大金额
```

### B. 监控指标

```
# 核心业务指标
- order_create_qps           # 订单创建QPS
- order_cancel_qps           # 订单撤单QPS
- trade_execute_qps          # 成交执行QPS
- order_latency_p99          # 订单延迟P99
- trade_latency_p99          # 成交延迟P99

# 资金安全指标
- fund_discrepancy_count     # 资金差异数量
- fund_discrepancy_amount    # 资金差异金额
- position_discrepancy_count # 持仓差异数量
- tcc_success_rate           # TCC事务成功率
- saga_success_rate          # Saga事务成功率

# 系统指标
- db_connection_count        # 数据库连接数
- db_query_latency           # 数据库查询延迟
- redis_hit_rate             # Redis命中率
- kafka_lag                  # Kafka消费延迟
- goroutine_count            # Goroutine数量
```

---

**文档版本**: v1.0  
**最后更新**: 2024-03-15  
**作者**: Trading System Team
