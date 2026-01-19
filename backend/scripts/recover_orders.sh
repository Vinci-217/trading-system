#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-password}"
DATABASE="${DATABASE:-stock_trader}"

REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"

LOG_FILE="order_recover_$(date +%Y%m%d_%H%M%S).log"

log() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${timestamp} [${level}] ${message}" | tee -a "$LOG_FILE"
}

info() { log "INFO" "${GREEN}$1${NC}"; }
warn() { log "WARN" "${YELLOW}$1${NC}"; }
error() { log "ERROR" "${RED}$1${NC}"; }
success() { log "SUCCESS" "${GREEN}$1${NC}"; }

echo_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
}

mysql_exec() {
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DATABASE" -N -e "$1" 2>/dev/null
}

mysql_exec_raw() {
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DATABASE" -N -e "$1"
}

redis_exec() {
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" "$@" 2>/dev/null
}

check_prerequisites() {
    echo_header "检查前置条件"
    
    info "检查数据库连接..."
    if ! mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "SELECT 1" > /dev/null 2>&1; then
        error "无法连接到数据库"
        exit 1
    fi
    success "数据库连接成功"
    
    info "检查Redis连接..."
    if ! redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping > /dev/null 2>&1; then
        warn "Redis连接失败, 部分功能将不可用"
    else
        success "Redis连接成功"
    fi
    
    info "检查必要的表是否存在..."
    local tables=$(mysql_exec "SHOW TABLES LIKE 'orders';")
    if [ -z "$tables" ]; then
        error "orders表不存在"
        exit 1
    fi
    success "必要的表存在"
}

backup_orders() {
    echo_header "备份订单数据"
    
    local backup_file="orders_backup_$(date +%Y%m%d_%H%M%S).sql"
    info "正在备份订单数据到 $backup_file..."
    
    mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DATABASE" orders tcc_transactions fund_locks > "$backup_file"
    
    if [ $? -eq 0 ]; then
        success "订单数据备份成功: $backup_file"
        echo "$backup_file"
    else
        error "订单数据备份失败"
        exit 1
    fi
}

find_stuck_orders() {
    echo_header "查找卡住的订单"
    
    info "查询状态异常超过24小时的订单..."
    
    local count=$(mysql_exec "
        SELECT COUNT(*) FROM orders 
        WHERE status = 'PENDING' 
        AND created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
    ")
    
    if [ "$count" -gt 0 ]; then
        warn "发现 $count 个卡住的订单"
        
        info "订单列表:"
        mysql_exec_raw "
            SELECT id, user_id, symbol, side, price, quantity, filled_quantity, status, created_at, updated_at
            FROM orders 
            WHERE status = 'PENDING' 
            AND created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
            LIMIT 100
        " | while read order_id user_id symbol side price quantity filled status created updated; do
            echo "   订单: $order_id | 用户: $user_id | 标的: $symbol | 方向: $side | 数量: $quantity | 价格: $price | 状态: $status | 创建时间: $created"
        done
    else
        success "没有发现卡住的订单"
    fi
    
    echo "$count"
}

find_pending_filled_orders() {
    echo_header "查找部分成交订单"
    
    info "查询部分成交但超过1小时未完成的订单..."
    
    local count=$(mysql_exec "
        SELECT COUNT(*) FROM orders 
        WHERE status = 'PARTIAL' 
        AND updated_at < DATE_SUB(NOW(), INTERVAL 1 HOUR)
    ")
    
    if [ "$count" -gt 0 ]; then
        warn "发现 $count 个部分成交订单"
        
        info "订单列表:"
        mysql_exec_raw "
            SELECT id, user_id, symbol, side, price, quantity, filled_quantity, status, updated_at
            FROM orders 
            WHERE status = 'PARTIAL' 
            AND updated_at < DATE_SUB(NOW(), INTERVAL 1 HOUR)
            LIMIT 100
        " | while read order_id user_id symbol side price quantity filled status updated; do
            echo "   订单: $order_id | 用户: $user_id | 标的: $symbol | 方向: $side | 成交量: $filled/$quantity | 更新时间: $updated"
        done
    else
        success "没有发现部分成交订单"
    fi
    
    echo "$count"
}

find_duplicate_orders() {
    echo_header "查找重复订单"
    
    info "查询同一用户的相同标的同方向的重复订单..."
    
    local count=$(mysql_exec "
        SELECT COUNT(*) FROM (
            SELECT user_id, symbol, side, price, COUNT(*) as cnt
            FROM orders 
            WHERE status = 'PENDING'
            GROUP BY user_id, symbol, side, price
            HAVING cnt > 1
        ) t
    ")
    
    if [ "$count" -gt 0 ]; then
        warn "发现 $count 组重复订单"
        
        info "重复订单组:"
        mysql_exec_raw "
            SELECT user_id, symbol, side, price, COUNT(*) as cnt
            FROM orders 
            WHERE status = 'PENDING'
            GROUP BY user_id, symbol, side, price
            HAVING cnt > 1
            LIMIT 50
        " | while read user_id symbol side price cnt; do
            echo "   用户: $user_id | 标的: $symbol | 方向: $side | 价格: $price | 重复数: $cnt"
        done
    else
        success "没有发现重复订单"
    fi
    
    echo "$count"
}

cancel_stuck_orders() {
    echo_header "取消卡住的订单"
    
    local auto_confirm="${1:-n}"
    
    info "开始取消卡住的订单..."
    
    local cancel_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, symbol, side, price, quantity
        FROM orders 
        WHERE status = 'PENDING' 
        AND created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
    " | while read order_id user_id symbol side price quantity; do
        cancel_count=$((cancel_count + 1))
        warn "取消订单: $order_id | 用户: $user_id | 标的: $symbol | 方向: $side | 数量: $quantity"
        
        info "更新订单状态..."
        mysql_exec "
            UPDATE orders 
            SET status = 'CANCELLED',
                updated_at = NOW(),
                remarks = '系统自动取消: 订单超时卡住'
            WHERE id = '$order_id'
        "
        
        info "释放冻结资金..."
        local lock_amount=$(mysql_exec "
            SELECT amount FROM fund_locks 
            WHERE order_id = '$order_id' AND status = 'LOCKED'
        ")
        
        if [ -n "$lock_amount" ]; then
            mysql_exec "
                UPDATE accounts a
                INNER JOIN fund_locks fl ON a.user_id = fl.user_id
                SET a.frozen_balance = a.frozen_balance - $lock_amount,
                    a.updated_at = NOW()
                WHERE fl.order_id = '$order_id' AND fl.status = 'LOCKED'
            "
            
            mysql_exec "
                UPDATE fund_locks 
                SET status = 'CANCELLED',
                    updated_at = NOW()
                WHERE order_id = '$order_id' AND status = 'LOCKED'
            "
            
            success "已释放冻结资金: $lock_amount"
        fi
        
        success "已取消订单 $order_id"
    done
    
    if [ $cancel_count -eq 0 ]; then
        success "没有需要取消的卡住订单"
    else
        success "取消了 $cancel_count 个卡住订单"
    fi
    
    echo "$cancel_count"
}

cancel_duplicate_orders() {
    echo_header "取消重复订单"
    
    info "开始取消重复订单(保留最早的订单)..."
    
    local cancel_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, symbol, side, price, created_at
        FROM orders 
        WHERE status = 'PENDING'
        AND (user_id, symbol, side, price) IN (
            SELECT user_id, symbol, side, price
            FROM orders 
            WHERE status = 'PENDING'
            GROUP BY user_id, symbol, side, price
            HAVING COUNT(*) > 1
        )
        AND id NOT IN (
            SELECT MIN(id)
            FROM orders 
            WHERE status = 'PENDING'
            GROUP BY user_id, symbol, side, price
        )
    " | while read order_id user_id symbol side price created; do
        cancel_count=$((cancel_count + 1))
        warn "取消重复订单: $order_id | 用户: $user_id | 标的: $symbol | 方向: $side | 创建时间: $created"
        
        mysql_exec "
            UPDATE orders 
            SET status = 'CANCELLED',
                updated_at = NOW(),
                remarks = '系统自动取消: 重复订单'
            WHERE id = '$order_id'
        "
        
        info "释放冻结资金..."
        local lock_amount=$(mysql_exec "SELECT amount FROM fund_locks WHERE order_id = '$order_id' AND status = 'LOCKED'")
        
        if [ -n "$lock_amount" ]; then
            mysql_exec "
                UPDATE accounts a
                INNER JOIN fund_locks fl ON a.user_id = fl.user_id
                SET a.frozen_balance = a.frozen_balance - $lock_amount,
                    a.updated_at = NOW()
                WHERE fl.order_id = '$order_id' AND fl.status = 'LOCKED'
            "
            
            mysql_exec "
                UPDATE fund_locks 
                SET status = 'CANCELLED'
                WHERE order_id = '$order_id' AND status = 'LOCKED'
            "
        fi
        
        success "已取消订单 $order_id"
    done
    
    if [ $cancel_count -eq 0 ]; then
        success "没有需要取消的重复订单"
    else
        success "取消了 $cancel_count 个重复订单"
    fi
    
    echo "$cancel_count"
}

force_fill_orders() {
    echo_header "强制完成订单"
    
    local order_id="${1:-}"
    
    if [ -z "$order_id" ]; then
        info "未指定订单ID, 查找所有可完成的订单..."
        
        mysql_exec_raw "
            SELECT id, user_id, symbol, side, price, quantity, filled_quantity
            FROM orders 
            WHERE status = 'PARTIAL' 
            AND filled_quantity > 0
            AND filled_quantity < quantity
        " | while read order_id user_id symbol side price quantity filled; do
            info "强制完成订单: $order_id | 用户: $user_id | 标的: $symbol | 方向: $side | 成交量: $filled/$quantity"
            
            local remaining=$((quantity - filled))
            
            mysql_exec "
                UPDATE orders 
                SET status = 'FILLED',
                    filled_quantity = quantity,
                    updated_at = NOW(),
                    remarks = '系统强制完成: 手动干预'
                WHERE id = '$order_id'
            "
            
            success "已完成订单 $order_id"
        done
    else
        info "强制完成指定订单: $order_id"
        
        local order_info=$(mysql_exec "SELECT user_id, symbol, side, price, quantity, filled_quantity FROM orders WHERE id = '$order_id'")
        
        if [ -z "$order_info" ]; then
            error "订单不存在: $order_id"
            return 1
        fi
        
        read user_id symbol side price quantity filled <<< "$order_info"
        
        mysql_exec "
            UPDATE orders 
            SET status = 'FILLED',
                filled_quantity = quantity,
                updated_at = NOW(),
                remarks = '系统强制完成: 手动干预'
            WHERE id = '$order_id'
        "
        
        success "已完成订单 $order_id"
    fi
}

fix_order_status() {
    echo_header "修复订单状态"
    
    info "修复状态不一致的订单..."
    
    local fix_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, symbol, side, price, quantity, filled_quantity, status
        FROM orders 
        WHERE status = 'FILLED' AND filled_quantity < quantity
    " | while read order_id user_id symbol side price quantity filled status; do
        fix_count=$((fix_count + 1))
        warn "修复订单状态: $order_id | 状态: $status | 成交量: $filled/$quantity"
        
        mysql_exec "
            UPDATE orders 
            SET status = 'PARTIAL'
            WHERE id = '$order_id'
        "
        
        success "已修复订单 $order_id"
    done
    
    if [ $fix_count -eq 0 ]; then
        success "没有需要修复的订单状态"
    else
        success "修复了 $fix_count 个订单状态"
    fi
    
    echo "$fix_count"
}

reset_tcc_for_orders() {
    echo_header "重置订单的TCC事务"
    
    info "查找订单关联的异常TCC事务..."
    
    local reset_count=0
    
    mysql_exec_raw "
        SELECT DISTINCT t.id, t.order_id, t.phase, t.status, t.retry_count
        FROM tcc_transactions t
        INNER JOIN orders o ON t.order_id = o.id
        WHERE t.status NOT IN ('SUCCESS', 'FAILED')
        AND t.created_at < DATE_SUB(NOW(), INTERVAL 2 HOUR)
    " | while read tx_id order_id phase status retry; do
        reset_count=$((reset_count + 1))
        warn "重置TCC事务: $tx_id | 订单: $order_id | 阶段: $phase | 状态: $status"
        
        mysql_exec "
            UPDATE tcc_transactions 
            SET status = 'FAILED',
                error_message = '自动标记: TCC事务超时'
            WHERE id = '$tx_id'
        "
        
        success "已重置TCC事务 $tx_id"
    done
    
    if [ $reset_count -eq 0 ]; then
        success "没有需要重置的TCC事务"
    else
        success "重置了 $reset_count 个TCC事务"
    fi
    
    echo "$reset_count"
}

recover_order_funds() {
    echo_header "恢复订单资金"
    
    local order_id="${1:-}"
    
    if [ -z "$order_id" ]; then
        info "恢复所有取消订单的资金..."
        
        local recover_count=0
        
        mysql_exec_raw "
            SELECT DISTINCT o.user_id, fl.id, fl.amount
            FROM orders o
            INNER JOIN fund_locks fl ON o.id = fl.order_id
            WHERE o.status = 'CANCELLED'
            AND fl.status = 'LOCKED'
        " | while read user_id lock_id amount; do
            recover_count=$((recover_count + 1))
            warn "恢复资金: 用户: $user_id | 锁ID: $lock_id | 金额: $amount"
            
            mysql_exec "
                UPDATE accounts 
                SET frozen_balance = frozen_balance - $amount,
                    updated_at = NOW()
                WHERE user_id = '$user_id'
            "
            
            mysql_exec "
                UPDATE fund_locks 
                SET status = 'CANCELLED'
                WHERE id = '$lock_id'
            "
            
            success "已恢复资金 $amount"
        done
        
        if [ $recover_count -eq 0 ]; then
            success "没有需要恢复的资金"
        else
            success "恢复了 $recover_count 个资金锁"
        fi
    else
        info "恢复指定订单的资金: $order_id"
        
        local lock_info=$(mysql_exec "SELECT user_id, amount FROM fund_locks WHERE order_id = '$order_id' AND status = 'LOCKED'")
        
        if [ -n "$lock_info" ]; then
            read user_id amount <<< "$lock_info"
            
            mysql_exec "
                UPDATE accounts 
                SET frozen_balance = frozen_balance - $amount,
                    updated_at = NOW()
                WHERE user_id = '$user_id'
            "
            
            mysql_exec "
                UPDATE fund_locks 
                SET status = 'CANCELLED'
                WHERE order_id = '$order_id' AND status = 'LOCKED'
            "
            
            success "已恢复资金 $amount"
        else
            warn "没有找到该订单的锁定资金"
        fi
    fi
}

sync_order_status() {
    echo_header "同步订单状态到Redis"
    
    info "同步所有PENDING和FILLED订单到Redis..."
    
    local sync_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, symbol, status, updated_at
        FROM orders 
        WHERE status IN ('PENDING', 'PARTIAL', 'FILLED')
    " | while read order_id user_id symbol status updated; do
        sync_count=$((sync_count + 1))
        
        redis_exec SET "order:$order_id" "{\"id\":\"$order_id\",\"user_id\":\"$user_id\",\"symbol\":\"$symbol\",\"status\":\"$status\",\"updated_at\":\"$updated\"}" EX 86400
    done
    
    success "同步了 $sync_count 个订单到Redis"
    
    echo "$sync_count"
}

show_order_status() {
    echo_header "显示订单状态统计"
    
    info "订单状态统计:"
    mysql_exec "
        SELECT 
            status as 状态,
            COUNT(*) as 数量,
            SUM(quantity) as 总数量,
            SUM(filled_quantity) as 总成交量
        FROM orders 
        GROUP BY status
        ORDER BY FIELD(status, 'PENDING', 'PARTIAL', 'FILLED', 'CANCELLED', 'REJECTED')
    " | while read status count total filled; do
        echo "   $status: $count 个订单 (总数量: $total, 成交量: $filled)"
    done
    
    info "今日订单统计:"
    local today=$(date '+%Y-%m-%d')
    mysql_exec "
        SELECT 
            status as 状态,
            COUNT(*) as 数量
        FROM orders 
        WHERE DATE(created_at) = '$today'
        GROUP BY status
    " | while read status count; do
        echo "   $status: $count 个"
    done
}

show_help() {
    echo_header "帮助信息"
    
    echo "用法: $0 [命令] [参数]"
    echo ""
    echo "可用命令:"
    echo "  check               - 检查所有订单问题"
    echo "  backup              - 备份订单数据"
    echo "  find-stuck          - 查找卡住的订单"
    echo "  find-partial        - 查找部分成交订单"
    echo "  find-duplicate      - 查找重复订单"
    echo "  cancel-stuck [y]    - 取消卡住的订单 (可指定y自动确认)"
    echo "  cancel-duplicate    - 取消重复订单(保留最早的)"
    echo "  force-fill [order]  - 强制完成订单(可指定订单ID)"
    echo "  fix-status          - 修复订单状态不一致"
    echo "  reset-tcc           - 重置异常TCC事务"
    echo "  recover [order]     - 恢复订单资金(可指定订单ID)"
    echo "  sync-redis          - 同步订单状态到Redis"
    echo "  status              - 显示订单状态统计"
    echo "  all                 - 执行所有修复操作"
    echo "  help                - 显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  DB_HOST       - 数据库主机 (默认: localhost)"
    echo "  DB_PORT       - 数据库端口 (默认: 3306)"
    echo "  DB_USER       - 数据库用户 (默认: root)"
    echo "  DB_PASSWORD   - 数据库密码 (默认: password)"
    echo "  DATABASE      - 数据库名 (默认: stock_trader)"
    echo "  REDIS_HOST    - Redis主机 (默认: localhost)"
    echo "  REDIS_PORT    - Redis端口 (默认: 6379)"
    echo ""
}

main() {
    local command="${1:-help}"
    local param="${2:-}"
    
    echo "========================================"
    echo "  证券交易系统订单恢复工具"
    echo "  $(date '+%Y-%m-%d %H:%M:%S')"
    echo "========================================"
    
    check_prerequisites
    
    case "$command" in
        check)
            find_stuck_orders
            find_pending_filled_orders
            find_duplicate_orders
            ;;
        backup)
            backup_orders
            ;;
        find-stuck)
            find_stuck_orders
            ;;
        find-partial)
            find_pending_filled_orders
            ;;
        find-duplicate)
            find_duplicate_orders
            ;;
        cancel-stuck)
            cancel_stuck_orders "$param"
            ;;
        cancel-duplicate)
            cancel_duplicate_orders
            ;;
        force-fill)
            force_fill_orders "$param"
            ;;
        fix-status)
            fix_order_status
            ;;
        reset-tcc)
            reset_tcc_for_orders
            ;;
        recover)
            recover_order_funds "$param"
            ;;
        sync-redis)
            sync_order_status
            ;;
        status)
            show_order_status
            ;;
        all)
            backup_orders
            find_stuck_orders
            find_pending_filled_orders
            find_duplicate_orders
            cancel_stuck_orders "y"
            cancel_duplicate_orders
            force_fill_orders
            fix_order_status
            reset_tcc_for_orders
            sync_order_status
            show_order_status
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
    
    echo ""
    success "操作完成"
}

main "$@"
