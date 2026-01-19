#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-password}"
DATABASE="${DATABASE:-stock_trader}"

LOG_FILE="fund_fix_$(date +%Y%m%d_%H%M%S).log"

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

check_prerequisites() {
    echo_header "检查前置条件"
    
    info "检查数据库连接..."
    if ! mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "SELECT 1" > /dev/null 2>&1; then
        error "无法连接到数据库"
        exit 1
    fi
    success "数据库连接成功"
    
    info "检查必要的表是否存在..."
    local tables=$(mysql_exec "SHOW TABLES LIKE 'accounts';")
    if [ -z "$tables" ]; then
        error "accounts表不存在"
        exit 1
    fi
    success "必要的表存在"
    
    info "检查Docker服务状态..."
    if command -v docker &> /dev/null; then
        if docker info &> /dev/null; then
            success "Docker服务运行正常"
        else
            warn "Docker服务未运行或无法访问"
        fi
    else
        warn "Docker未安装"
    fi
}

backup_database() {
    echo_header "备份数据库"
    
    local backup_file="fund_backup_$(date +%Y%m%d_%H%M%S).sql"
    info "正在备份数据库到 $backup_file..."
    
    mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DATABASE" accounts orders positions fund_locks tcc_transactions > "$backup_file"
    
    if [ $? -eq 0 ]; then
        success "数据库备份成功: $backup_file"
        echo "$backup_file"
    else
        error "数据库备份失败"
        exit 1
    fi
}

check_fund_discrepancies() {
    echo_header "检查资金差异"
    
    info "查询所有用户的资金数据..."
    
    local discrepancies=0
    
    mysql_exec_raw "
        SELECT 
            a.user_id,
            a.cash_balance,
            a.frozen_balance,
            (a.cash_balance + a.frozen_balance) as total_balance,
            COALESCE(SUM(
                CASE 
                    WHEN o.status = 'PENDING' AND o.side = 'BUY' 
                    THEN o.price * o.quantity 
                    ELSE 0 
                END
            ), 0) as pending_buy_amount,
            COALESCE(SUM(
                CASE 
                    WHEN o.status = 'PENDING' AND o.side = 'SELL' 
                    THEN o.quantity 
                    ELSE 0 
                END
            ), 0) as pending_sell_quantity
        FROM accounts a
        LEFT JOIN orders o ON a.user_id = o.user_id AND o.status = 'PENDING'
        GROUP BY a.user_id, a.cash_balance, a.frozen_balance
        HAVING ABS(total_balance - pending_buy_amount) > 0.01
    " | while read user_id cash_balance frozen total pending_buy; do
        discrepancies=$((discrepancies + 1))
        warn "发现资金差异: 用户=$user_id, 现金=$cash_balance, 冻结=$frozen, 待买入=$pending_buy"
    done
    
    if [ $discrepancies -eq 0 ]; then
        success "未发现资金差异"
    else
        warn "发现 $discrepancies 个资金差异需要处理"
    fi
}

fix_frozen_balance() {
    echo_header "修复冻结资金问题"
    
    info "查找冻结资金大于可用资金的用户..."
    
    local fix_count=0
    
    mysql_exec_raw "
        SELECT user_id, cash_balance, frozen_balance 
        FROM accounts 
        WHERE frozen_balance > cash_balance
    " | while read user_id cash frozen; do
        fix_count=$((fix_count + 1))
        warn "用户 $user_id 冻结资金异常: 现金=$cash, 冻结=$frozen"
        
        info "修复冻结资金..."
        mysql_exec "
            UPDATE accounts 
            SET frozen_balance = cash_balance 
            WHERE user_id = '$user_id'
        "
        success "已修复用户 $user_id 的冻结资金"
    done
    
    if [ $fix_count -eq 0 ]; then
        success "没有需要修复的冻结资金问题"
    else
        success "修复了 $fix_count 个冻结资金问题"
    fi
}

fix_negative_balance() {
    echo_header "修复负数余额"
    
    info "查找负数余额的用户..."
    
    local fix_count=0
    
    mysql_exec_raw "
        SELECT user_id, cash_balance, frozen_balance 
        FROM accounts 
        WHERE cash_balance < 0 OR frozen_balance < 0
    " | while read user_id cash frozen; do
        fix_count=$((fix_count + 1))
        warn "用户 $user_id 余额异常: 现金=$cash, 冻结=$frozen"
        
        info "修复负数余额..."
        if [ "$(echo "$cash < 0" | bc)" -eq 1 ]; then
            mysql_exec "
                UPDATE accounts 
                SET cash_balance = 0 
                WHERE user_id = '$user_id'
            "
            success "已修复用户 $user_id 的负数现金"
        fi
        
        if [ "$(echo "$frozen < 0" | bc)" -eq 1 ]; then
            mysql_exec "
                UPDATE accounts 
                SET frozen_balance = 0 
                WHERE user_id = '$user_id'
            "
            success "已修复用户 $user_id 的负数冻结资金"
        fi
    done
    
    if [ $fix_count -eq 0 ]; then
        success "没有需要修复的负数余额问题"
    else
        success "修复了 $fix_count 个负数余额问题"
    fi
}

fix_orphaned_locks() {
    echo_header "修复孤立锁记录"
    
    info "查找没有对应订单的资金锁..."
    
    local fix_count=0
    
    mysql_exec_raw "
        SELECT fl.id, fl.user_id, fl.order_id, fl.amount, fl.status
        FROM fund_locks fl
        LEFT JOIN orders o ON fl.order_id = o.id
        WHERE o.id IS NULL AND fl.status = 'LOCKED'
    " | while read lock_id user_id order_id amount status; do
        fix_count=$((fix_count + 1))
        warn "发现孤立锁: 锁ID=$lock_id, 用户=$user_id, 订单=$order_id, 金额=$amount"
        
        info "修复孤立锁..."
        mysql_exec "
            UPDATE fund_locks 
            SET status = 'CANCELLED' 
            WHERE id = '$lock_id'
        "
        success "已取消孤立锁 $lock_id 并释放资金"
    done
    
    if [ $fix_count -eq 0 ]; then
        success "没有需要修复的孤立锁"
    else
        success "修复了 $fix_count 个孤立锁"
    fi
}

fix_expired_locks() {
    echo_header "修复过期锁记录"
    
    info "查找过期的资金锁..."
    
    local fix_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, order_id, amount, expires_at
        FROM fund_locks 
        WHERE status = 'LOCKED' AND expires_at < NOW()
    " | while read lock_id user_id order_id amount expires; do
        fix_count=$((fix_count + 1))
        warn "发现过期锁: 锁ID=$lock_id, 用户=$user_id, 订单=$order_id, 金额=$amount, 过期时间=$expires"
        
        info "修复过期锁..."
        mysql_exec "
            UPDATE fund_locks 
            SET status = 'EXPIRED' 
            WHERE id = '$lock_id'
        "
        success "已将过期锁 $lock_id 标记为过期"
    done
    
    if [ $fix_count -eq 0 ]; then
        success "没有需要修复的过期锁"
    else
        success "修复了 $fix_count 个过期锁"
    fi
}

reconcile_all_accounts() {
    echo_header "对账所有账户"
    
    info "开始全量对账..."
    
    local total_accounts=$(mysql_exec "SELECT COUNT(*) FROM accounts")
    info "总账户数: $total_accounts"
    
    local check_count=0
    local discrepancy_count=0
    
    mysql_exec_raw "
        SELECT user_id, cash_balance, frozen_balance 
        FROM accounts 
        WHERE cash_balance < 0 OR frozen_balance < 0 OR frozen_balance > cash_balance
    " | while read user_id cash frozen; do
        discrepancy_count=$((discrepancy_count + 1))
        check_count=$((check_count + 1))
        warn "账户异常: 用户=$user_id, 现金=$cash, 冻结=$frozen"
    done
    
    mysql_exec_raw "
        SELECT COUNT(DISTINCT user_id)
        FROM orders 
        WHERE status = 'PENDING'
    " | while read pending_users; do
        check_count=$((check_count + pending_users))
    done
    
    success "对账完成: 检查 $check_count 个账户, 发现 $discrepancy_count 个异常"
    
    echo "$discrepancy_count"
}

reset_stuck_orders() {
    echo_header "重置卡住的订单"
    
    info "查找状态异常的订单..."
    
    local reset_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, status, created_at 
        FROM orders 
        WHERE status = 'PENDING' 
        AND created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
    " | while read order_id user_id status created; do
        reset_count=$((reset_count + 1))
        warn "发现卡住的订单: ID=$order_id, 用户=$user_id, 状态=$status, 创建时间=$created"
        
        info "重置订单状态..."
        mysql_exec "
            UPDATE orders 
            SET status = 'CANCELLED', 
                updated_at = NOW(),
                remarks = '系统自动取消: 订单超时卡住'
            WHERE id = '$order_id'
        "
        success "已取消订单 $order_id"
    done
    
    if [ $reset_count -eq 0 ]; then
        success "没有需要重置的卡住订单"
    else
        success "重置了 $reset_count 个卡住的订单"
    fi
}

recover_tcc_transactions() {
    echo_header "恢复TCC事务"
    
    info "查找未完成的TCC事务..."
    
    local recover_count=0
    
    mysql_exec_raw "
        SELECT id, user_id, order_id, phase, status, retry_count
        FROM tcc_transactions 
        WHERE status NOT IN ('SUCCESS', 'FAILED')
        AND created_at < DATE_SUB(NOW(), INTERVAL 1 HOUR)
    " | while read tx_id user_id order_id phase status retry; do
        recover_count=$((recover_count + 1))
        warn "发现未完成TCC事务: ID=$tx_id, 用户=$user_id, 阶段=$phase, 状态=$status, 重试次数=$retry"
        
        if [ "$retry" -lt 3 ]; then
            info "增加重试次数..."
            mysql_exec "
                UPDATE tcc_transactions 
                SET retry_count = retry_count + 1 
                WHERE id = '$tx_id'
            "
            success "已增加事务 $tx_id 的重试次数"
        else
            info "标记事务为失败..."
            mysql_exec "
                UPDATE tcc_transactions 
                SET status = 'FAILED', 
                    error_message = '自动标记: 超过最大重试次数'
                WHERE id = '$tx_id'
            "
            success "已将事务 $tx_id 标记为失败"
        fi
    done
    
    if [ $recover_count -eq 0 ]; then
        success "没有需要恢复的TCC事务"
    else
        success "处理了 $recover_count 个TCC事务"
    fi
}

fix_position_discrepancies() {
    echo_header "修复持仓差异"
    
    info "检查持仓与订单的一致性..."
    
    local fix_count=0
    
    mysql_exec_raw "
        SELECT p.user_id, p.symbol, p.quantity, p.avg_cost
        FROM positions p
    " | while read user_id symbol quantity avg_cost; do
        local filled_sell=$(mysql_exec "
            SELECT COALESCE(SUM(quantity), 0)
            FROM orders 
            WHERE user_id = '$user_id' 
            AND symbol = '$symbol' 
            AND side = 'SELL' 
            AND status = 'FILLED'
        ")
        
        local filled_buy=$(mysql_exec "
            SELECT COALESCE(SUM(quantity), 0)
            FROM orders 
            WHERE user_id = '$user_id' 
            AND symbol = '$symbol' 
            AND side = 'BUY' 
            AND status = 'FILLED'
        ")
        
        local expected_quantity=$((filled_buy - filled_sell))
        
        if [ "$quantity" -ne "$expected_quantity" ]; then
            fix_count=$((fix_count + 1))
            warn "持仓差异: 用户=$user_id, 标的=$symbol, 当前持仓=$quantity, 预期持仓=$expected_quantity"
            
            info "修复持仓差异..."
            mysql_exec "
                UPDATE positions 
                SET quantity = $expected_quantity,
                    updated_at = NOW()
                WHERE user_id = '$user_id' AND symbol = '$symbol'
            "
            success "已修复用户 $user_id 标的 $symbol 的持仓"
        fi
    done
    
    if [ $fix_count -eq 0 ]; then
        success "没有需要修复的持仓差异"
    else
        success "修复了 $fix_count 个持仓差异"
    fi
}

run_full_check() {
    echo_header "运行完整检查"
    
    local issues=0
    
    info "1. 检查负数余额..."
    local negative_balance=$(mysql_exec "SELECT COUNT(*) FROM accounts WHERE cash_balance < 0 OR frozen_balance < 0")
    issues=$((issues + negative_balance))
    echo "   负数余额账户数: $negative_balance"
    
    info "2. 检查冻结资金异常..."
    local frozen_issue=$(mysql_exec "SELECT COUNT(*) FROM accounts WHERE frozen_balance > cash_balance")
    issues=$((issues + frozen_issue))
    echo "   冻结异常账户数: $frozen_issue"
    
    info "3. 检查孤立锁..."
    local orphan_locks=$(mysql_exec "
        SELECT COUNT(*) FROM fund_locks fl
        LEFT JOIN orders o ON fl.order_id = o.id
        WHERE o.id IS NULL AND fl.status = 'LOCKED'
    ")
    issues=$((issues + orphan_locks))
    echo "   孤立锁数量: $orphan_locks"
    
    info "4. 检查过期锁..."
    local expired_locks=$(mysql_exec "
        SELECT COUNT(*) FROM fund_locks 
        WHERE status = 'LOCKED' AND expires_at < NOW()
    ")
    issues=$((issues + expired_locks))
    echo "   过期锁数量: $expired_locks"
    
    info "5. 检查卡住订单..."
    local stuck_orders=$(mysql_exec "
        SELECT COUNT(*) FROM orders 
        WHERE status = 'PENDING' 
        AND created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
    ")
    issues=$((issues + stuck_orders))
    echo "   卡住订单数量: $stuck_orders"
    
    info "6. 检查未完成TCC事务..."
    local pending_tcc=$(mysql_exec "
        SELECT COUNT(*) FROM tcc_transactions 
        WHERE status NOT IN ('SUCCESS', 'FAILED')
    ")
    issues=$((issues + pending_tcc))
    echo "   未完成TCC事务数: $pending_tcc"
    
    if [ $issues -eq 0 ]; then
        success "完整检查通过, 未发现任何问题"
    else
        warn "完整检查完成, 发现 $issues 个问题需要处理"
    fi
    
    echo "$issues"
}

show_status() {
    echo_header "显示当前状态"
    
    info "账户统计:"
    mysql_exec "
        SELECT 
            '总账户数' as label, COUNT(*) as value FROM accounts
        UNION ALL
        SELECT 
            '负数余额账户', COUNT(*) FROM accounts WHERE cash_balance < 0
        UNION ALL
        SELECT 
            '冻结异常账户', COUNT(*) FROM accounts WHERE frozen_balance > cash_balance
    " | while read label value; do
        echo "   $label: $value"
    done
    
    info "订单统计:"
    mysql_exec "
        SELECT 
            '总订单数' as label, COUNT(*) as value FROM orders
        UNION ALL
        SELECT 
            '待处理订单', COUNT(*) FROM orders WHERE status = 'PENDING'
        UNION ALL
        SELECT 
            '已完成订单', COUNT(*) FROM orders WHERE status = 'FILLED'
    " | while read label value; do
        echo "   $label: $value"
    done
    
    info "资金锁统计:"
    mysql_exec "
        SELECT 
            '总锁数' as label, COUNT(*) as value FROM fund_locks
        UNION ALL
        SELECT 
            '活跃锁', COUNT(*) FROM fund_locks WHERE status = 'LOCKED'
        UNION ALL
        SELECT 
            '已释放锁', COUNT(*) FROM fund_locks WHERE status IN ('CONFIRMED', 'CANCELLED')
    " | while read label value; do
        echo "   $label: $value"
    done
}

show_help() {
    echo_header "帮助信息"
    
    echo "用法: $0 [命令]"
    echo ""
    echo "可用命令:"
    echo "  check               - 检查资金差异"
    echo "  backup              - 备份数据库"
    echo "  fix-frozen          - 修复冻结资金问题"
    echo "  fix-negative        - 修复负数余额"
    echo "  fix-orphan-locks    - 修复孤立锁"
    echo "  fix-expired-locks   - 修复过期锁"
    echo "  fix-positions       - 修复持仓差异"
    echo "  reconcile           - 对账所有账户"
    echo "  reset-stuck         - 重置卡住的订单"
    echo "  recover-tcc         - 恢复TCC事务"
    echo "  full-check          - 运行完整检查"
    echo "  status              - 显示当前状态"
    echo "  all                 - 执行所有修复操作"
    echo "  help                - 显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  DB_HOST     - 数据库主机 (默认: localhost)"
    echo "  DB_PORT     - 数据库端口 (默认: 3306)"
    echo "  DB_USER     - 数据库用户 (默认: root)"
    echo "  DB_PASSWORD - 数据库密码 (默认: password)"
    echo "  DATABASE    - 数据库名 (默认: stock_trader)"
    echo ""
}

main() {
    local command="${1:-help}"
    
    echo "========================================"
    echo "  证券交易系统资金修复工具"
    echo "  $(date '+%Y-%m-%d %H:%M:%S')"
    echo "========================================"
    
    check_prerequisites
    
    case "$command" in
        check)
            check_fund_discrepancies
            ;;
        backup)
            backup_database
            ;;
        fix-frozen)
            fix_frozen_balance
            ;;
        fix-negative)
            fix_negative_balance
            ;;
        fix-orphan-locks)
            fix_orphaned_locks
            ;;
        fix-expired-locks)
            fix_expired_locks
            ;;
        fix-positions)
            fix_position_discrepancies
            ;;
        reconcile)
            reconcile_all_accounts
            ;;
        reset-stuck)
            reset_stuck_orders
            ;;
        recover-tcc)
            recover_tcc_transactions
            ;;
        full-check)
            run_full_check
            ;;
        status)
            show_status
            ;;
        all)
            backup_database
            fix_frozen_balance
            fix_negative_balance
            fix_orphaned_locks
            fix_expired_locks
            fix_position_discrepancies
            reset_stuck_orders
            recover_tcc_transactions
            run_full_check
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
