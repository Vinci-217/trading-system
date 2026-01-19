package repository

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"stock_trader/internal/config"
	"stock_trader/internal/model"
)

type DB struct {
	DB   *gorm.DB
	log  *zap.Logger
}

func NewDB(cfg *config.DatabaseConfig) (*DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库实例失败: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetimeDuration())

	return &DB{DB: db, log: nil}, nil
}

func (d *DB) SetLogger(log *zap.Logger) {
	d.log = log
}

func (d *DB) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type Repository struct {
	db *DB
}

func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) AutoMigrate() error {
	return r.db.DB.AutoMigrate(
		&model.User{},
		&model.Account{},
		&model.Position{},
		&model.Order{},
		&model.Trade{},
		&model.Stock{},
		&model.ReconciliationRecord{},
		&model.ReconciliationIssue{},
	)
}

func (r *Repository) CreateUser(ctx context.Context, user *model.User) error {
	result := r.db.DB.WithContext(ctx).Create(user)
	if result.Error != nil {
		return fmt.Errorf("创建用户失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	result := r.db.DB.WithContext(ctx).Where("username = ?", username).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	result := r.db.DB.WithContext(ctx).First(&user, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (r *Repository) CreateAccount(ctx context.Context, account *model.Account) error {
	result := r.db.DB.WithContext(ctx).Create(account)
	if result.Error != nil {
		return fmt.Errorf("创建账户失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetAccountByUserID(ctx context.Context, userID int64) (*model.Account, error) {
	var account model.Account
	result := r.db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&account)
	if result.Error != nil {
		return nil, result.Error
	}
	return &account, nil
}

func (r *Repository) UpdateAccount(ctx context.Context, account *model.Account) error {
	result := r.db.DB.WithContext(ctx).Save(account)
	if result.Error != nil {
		return fmt.Errorf("更新账户失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetAccountForUpdate(ctx context.Context, userID int64) (*model.Account, error) {
	var account model.Account
	result := r.db.DB.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&account)
	if result.Error != nil {
		return nil, result.Error
	}
	return &account, nil
}

func (r *Repository) CreatePosition(ctx context.Context, position *model.Position) error {
	result := r.db.DB.WithContext(ctx).Create(position)
	if result.Error != nil {
		return fmt.Errorf("创建持仓失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetPosition(ctx context.Context, userID int64, symbol string) (*model.Position, error) {
	var position model.Position
	result := r.db.DB.WithContext(ctx).Where("user_id = ? AND symbol = ?", userID, symbol).First(&position)
	if result.Error != nil {
		return nil, result.Error
	}
	return &position, nil
}

func (r *Repository) GetPositions(ctx context.Context, userID int64) ([]*model.Position, error) {
	var positions []*model.Position
	result := r.db.DB.WithContext(ctx).Where("user_id = ?", userID).Find(&positions)
	if result.Error != nil {
		return nil, result.Error
	}
	return positions, nil
}

func (r *Repository) UpdatePosition(ctx context.Context, position *model.Position) error {
	result := r.db.DB.WithContext(ctx).Save(position)
	if result.Error != nil {
		return fmt.Errorf("更新持仓失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) UpsertPosition(ctx context.Context, position *model.Position) error {
	result := r.db.DB.WithContext(ctx).Where("user_id = ? AND symbol = ?", position.UserID, position.Symbol).Assign(map[string]interface{}{
		"quantity":            position.Quantity,
		"available_quantity": position.AvailableQuantity,
		"cost_price":         position.CostPrice,
		"current_price":      position.CurrentPrice,
		"market_value":       position.MarketValue,
		"profit_loss":        position.ProfitLoss,
		"profit_loss_rate":   position.ProfitLossRate,
	}).FirstOrCreate(position)
	if result.Error != nil {
		return fmt.Errorf("创建或更新持仓失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) CreateOrder(ctx context.Context, order *model.Order) error {
	result := r.db.DB.WithContext(ctx).Create(order)
	if result.Error != nil {
		return fmt.Errorf("创建订单失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetOrder(ctx context.Context, orderID string) (*model.Order, error) {
	var order model.Order
	result := r.db.DB.WithContext(ctx).Where("order_id = ?", orderID).First(&order)
	if result.Error != nil {
		return nil, result.Error
	}
	return &order, nil
}

func (r *Repository) GetOrderForUpdate(ctx context.Context, orderID string) (*model.Order, error) {
	var order model.Order
	result := r.db.DB.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id = ?", orderID).First(&order)
	if result.Error != nil {
		return nil, result.Error
	}
	return &order, nil
}

func (r *Repository) UpdateOrder(ctx context.Context, order *model.Order) error {
	result := r.db.DB.WithContext(ctx).Save(order)
	if result.Error != nil {
		return fmt.Errorf("更新订单失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) ListOrders(ctx context.Context, userID int64, symbol, status string, limit, offset int) ([]*model.Order, int64, error) {
	var orders []*model.Order
	var total int64

	query := r.db.DB.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	result := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&orders)
	if result.Error != nil {
		return nil, 0, result.Error
	}
	return orders, total, nil
}

func (r *Repository) CreateTrade(ctx context.Context, trade *model.Trade) error {
	result := r.db.DB.WithContext(ctx).Create(trade)
	if result.Error != nil {
		return fmt.Errorf("创建成交失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) CreateTrades(ctx context.Context, trades []*model.Trade) error {
	if len(trades) == 0 {
		return nil
	}
	result := r.db.DB.WithContext(ctx).CreateInBatches(trades, 100)
	if result.Error != nil {
		return fmt.Errorf("批量创建成交失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetTrades(ctx context.Context, userID int64, symbol string, limit, offset int) ([]*model.Trade, int64, error) {
	var trades []*model.Trade
	var total int64

	query := r.db.DB.WithContext(ctx).Model(&model.Trade{}).Where("user_id = ?", userID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	result := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&trades)
	if result.Error != nil {
		return nil, 0, result.Error
	}
	return trades, total, nil
}

func (r *Repository) GetTradesByOrderID(ctx context.Context, orderID string) ([]*model.Trade, error) {
	var trades []*model.Trade
	result := r.db.DB.WithContext(ctx).Where("order_id = ?", orderID).Find(&trades)
	if result.Error != nil {
		return nil, result.Error
	}
	return trades, nil
}

func (r *Repository) GetStock(ctx context.Context, symbol string) (*model.Stock, error) {
	var stock model.Stock
	result := r.db.DB.WithContext(ctx).Where("symbol = ?", symbol).First(&stock)
	if result.Error != nil {
		return nil, result.Error
	}
	return &stock, nil
}

func (r *Repository) GetAllStocks(ctx context.Context) ([]*model.Stock, error) {
	var stocks []*model.Stock
	result := r.db.DB.WithContext(ctx).Where("status = ?", model.StockStatusActive).Find(&stocks)
	if result.Error != nil {
		return nil, result.Error
	}
	return stocks, nil
}

func (r *Repository) CreateReconciliationRecord(ctx context.Context, record *model.ReconciliationRecord) error {
	result := r.db.DB.WithContext(ctx).Create(record)
	if result.Error != nil {
		return fmt.Errorf("创建对账记录失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetReconciliationRecords(ctx context.Context, startTime, endTime time.Time) ([]*model.ReconciliationRecord, error) {
	var records []*model.ReconciliationRecord
	query := r.db.DB.WithContext(ctx).Where("check_time >= ? AND check_time <= ?", startTime, endTime)
	result := query.Order("check_time DESC").Find(&records)
	if result.Error != nil {
		return nil, result.Error
	}
	return records, nil
}

func (r *Repository) CreateReconciliationIssue(ctx context.Context, issue *model.ReconciliationIssue) error {
	result := r.db.DB.WithContext(ctx).Create(issue)
	if result.Error != nil {
		return fmt.Errorf("创建对账差异失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) CreateReconciliationIssues(ctx context.Context, issues []*model.ReconciliationIssue) error {
	if len(issues) == 0 {
		return nil
	}
	result := r.db.DB.WithContext(ctx).CreateInBatches(issues, 100)
	if result.Error != nil {
		return fmt.Errorf("批量创建对账差异失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetReconciliationIssues(ctx context.Context, reportID, status string) ([]*model.ReconciliationIssue, error) {
	var issues []*model.ReconciliationIssue
	query := r.db.DB.WithContext(ctx)
	if reportID != "" {
		query = query.Where("report_id = ?", reportID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	result := query.Order("created_at DESC").Find(&issues)
	if result.Error != nil {
		return nil, result.Error
	}
	return issues, nil
}

func (r *Repository) UpdateReconciliationIssue(ctx context.Context, issue *model.ReconciliationIssue) error {
	result := r.db.DB.WithContext(ctx).Save(issue)
	if result.Error != nil {
		return fmt.Errorf("更新对账差异失败: %w", result.Error)
	}
	return nil
}

func (r *Repository) GetAllAccounts(ctx context.Context) ([]*model.Account, error) {
	var accounts []*model.Account
	result := r.db.DB.WithContext(ctx).Find(&accounts)
	if result.Error != nil {
		return nil, result.Error
	}
	return accounts, nil
}

func (r *Repository) GetAllPositions(ctx context.Context) ([]*model.Position, error) {
	var positions []*model.Position
	result := r.db.DB.WithContext(ctx).Find(&positions)
	if result.Error != nil {
		return nil, result.Error
	}
	return positions, nil
}

func (r *Repository) GetAllTrades(ctx context.Context, startTime, endTime time.Time) ([]*model.Trade, error) {
	var trades []*model.Trade
	result := r.db.DB.WithContext(ctx).Where("created_at >= ? AND created_at <= ?", startTime, endTime).Find(&trades)
	if result.Error != nil {
		return nil, result.Error
	}
	return trades, nil
}

func (r *Repository) GetAllOrders(ctx context.Context, startTime, endTime time.Time) ([]*model.Order, error) {
	var orders []*model.Order
	result := r.db.DB.WithContext(ctx).Where("created_at >= ? AND created_at <= ?", startTime, endTime).Find(&orders)
	if result.Error != nil {
		return nil, result.Error
	}
	return orders, nil
}

func (r *Repository) GetOrdersByUserIDAndTime(ctx context.Context, userID int64, startTime, endTime time.Time) ([]*model.Order, error) {
	var orders []*model.Order
	result := r.db.DB.WithContext(ctx).Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, startTime, endTime).Find(&orders)
	if result.Error != nil {
		return nil, result.Error
	}
	return orders, nil
}

func (r *Repository) GetTradesByUserIDAndTime(ctx context.Context, userID int64, startTime, endTime time.Time) ([]*model.Trade, error) {
	var trades []*model.Trade
	result := r.db.DB.WithContext(ctx).Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, startTime, endTime).Find(&trades)
	if result.Error != nil {
		return nil, result.Error
	}
	return trades, nil
}

import "gorm.io/gorm/clause"
