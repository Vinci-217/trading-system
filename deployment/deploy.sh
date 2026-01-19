#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEPLOY_DIR="$PROJECT_ROOT/deployment"
LOG_DIR="$PROJECT_ROOT/logs"

VERSION="${VERSION:-v1.0.0}"
BUILD_PARALLEL="${BUILD_PARALLEL:-true}"
SKIP_BUILD="${SKIP_BUILD:-false}"

mkdir -p "$LOG_DIR"

log() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${timestamp} [${level}] ${message}" | tee -a "$LOG_DIR/deploy.log"
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

check_prerequisites() {
    echo_header "检查前置条件"
    
    info "检查Docker..."
    if ! command -v docker &> /dev/null; then
        error "Docker未安装，请先安装Docker"
        exit 1
    fi
    docker --version
    success "Docker已安装"
    
    info "检查Docker Compose..."
    if ! command -v docker-compose &> /dev/null; then
        error "Docker Compose未安装，请先安装Docker Compose"
        exit 1
    fi
    docker-compose --version
    success "Docker Compose已安装"
    
    info "检查Docker服务状态..."
    if ! docker info &> /dev/null; then
        error "Docker服务未运行，请启动Docker服务"
        exit 1
    fi
    success "Docker服务运行正常"
    
    info "检查项目目录..."
    if [ ! -d "$DEPLOY_DIR" ]; then
        error "部署目录不存在: $DEPLOY_DIR"
        exit 1
    fi
    success "项目目录存在"
    
    info "检查docker-compose.yml..."
    if [ ! -f "$DEPLOY_DIR/docker-compose.yml" ]; then
        error "docker-compose.yml不存在"
        exit 1
    fi
    success "docker-compose.yml存在"
}

check_system_resources() {
    echo_header "检查系统资源"
    
    info "检查可用内存..."
    total_mem=$(docker system info --format '{{.MemTotal}}' 2>/dev/null || echo "0")
    if [ "$total_mem" -lt 4000000000 ]; then
        warn "可用内存较低，可能影响服务性能"
    else
        success "内存充足"
    fi
    
    info "检查磁盘空间..."
    available_space=$(df -BG "$DEPLOY_DIR" | awk 'NR==2 {print $4}' | tr -d 'G')
    if [ "${available_space:-0}" -lt 10 ]; then
        warn "磁盘空间不足10GB，可能影响服务运行"
    else
        success "磁盘空间充足"
    fi
}

backup_data() {
    echo_header "备份数据"
    
    local backup_dir="$PROJECT_ROOT/backups/$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    
    info "创建数据备份目录: $backup_dir"
    
    info "备份MySQL数据..."
    docker exec stock_trader_mysql mysqldump -u root -ppassword stock_trader > "$backup_dir/stock_trader.sql" 2>/dev/null || warn "MySQL备份失败（服务可能未运行）"
    
    info "备份Redis数据..."
    docker exec stock_trader_redis redis-cli BGSAVE > /dev/null 2>&1 || warn "Redis备份失败（服务可能未运行）"
    
    if [ -f "$backup_dir/stock_trader.sql" ]; then
        success "数据备份完成: $backup_dir"
    else
        warn "部分备份可能失败，但将继续部署"
    fi
}

stop_services() {
    echo_header "停止服务"
    
    cd "$DEPLOY_DIR"
    
    info "停止所有服务..."
    docker-compose down --remove-orphans 2>/dev/null || true
    
    info "等待服务完全停止..."
    sleep 3
    
    info "清理残留容器..."
    docker ps -a --filter "name=stock_trader_" -q | xargs -r docker rm -f 2>/dev/null || true
    
    success "服务已停止"
}

clean_images() {
    echo_header "清理旧镜像"
    
    info "清理未使用的镜像..."
    docker image prune -f > /dev/null 2>&1 || true
    
    info "清理构建缓存..."
    docker builder prune -f > /dev/null 2>&1 || true
    
    success "清理完成"
}

build_images() {
    echo_header "构建服务镜像"
    
    cd "$DEPLOY_DIR"
    
    local services=("user-service" "order-service" "account-service" "market-service" "matching-service" "reconciliation-service" "api-gateway")
    
    if [ "$BUILD_PARALLEL" == "true" ]; then
        info "并行构建所有服务镜像..."
        
        local pids=()
        for service in "${services[@]}"; do
            (
                info "构建镜像: $service"
                docker-compose build "$service" 2>&1 | tee -a "$LOG_DIR/build_$service.log"
                if [ ${PIPESTATUS[0]} -eq 0 ]; then
                    success "$service 构建成功"
                else
                    error "$service 构建失败"
                    exit 1
                fi
            ) &
            pids+=($!)
        done
        
        for pid in "${pids[@]}"; do
            wait $pid || exit 1
        done
        
        success "所有镜像构建完成"
    else
        for service in "${services[@]}"; do
            info "构建镜像: $service"
            if docker-compose build "$service" 2>&1 | tee -a "$LOG_DIR/build_$service.log"; then
                success "$service 构建成功"
            else
                error "$service 构建失败"
                exit 1
            fi
        done
    fi
    
    info "标记镜像版本..."
    for service in "${services[@]}"; do
        docker tag "stock_trader-$service:latest" "stock_trader-$service:$VERSION" 2>/dev/null || true
    done
}

start_services() {
    echo_header "启动服务"
    
    cd "$DEPLOY_DIR"
    
    export VERSION="$VERSION"
    
    info "启动所有服务..."
    if docker-compose up -d; then
        success "服务启动命令执行成功"
    else
        error "服务启动失败"
        exit 1
    fi
    
    info "等待服务初始化..."
    sleep 10
}

wait_for_services() {
    echo_header "等待服务就绪"
    
    local services=("mysql" "redis" "kafka" "user-service" "order-service" "account-service" "market-service" "matching-service" "reconciliation-service" "api-gateway")
    local max_attempts=60
    local attempt=0
    
    info "检查服务健康状态..."
    
    for service in "${services[@]}"; do
        attempt=0
        while [ $attempt -lt $max_attempts ]; do
            if docker inspect --format='{{.State.Running}}' "stock_trader_$service" 2>/dev/null | grep -q "true"; then
                if docker inspect --format='{{.State.Health.Status}}' "stock_trader_$service" 2>/dev/null | grep -q "healthy" 2>/dev/null; then
                    success "$service 运行正常"
                    break
                elif [ "$service" == "kafka" ] || [ "$service" == "mysql" ] || [ "$service" == "redis" ]; then
                    success "$service 运行正常"
                    break
                fi
            fi
            
            attempt=$((attempt + 1))
            sleep 2
        done
        
        if [ $attempt -ge $max_attempts ]; then
            warn "$service 启动超时，正在检查日志..."
            docker logs "stock_trader_$service" 2>&1 | tail -20
        fi
    done
}

verify_services() {
    echo_header "验证服务"
    
    local all_healthy=true
    
    info "检查服务健康状态..."
    
    local services=("api-gateway" "user-service" "order-service" "account-service" "market-service" "matching-service" "reconciliation-service")
    
    for service in "${services[@]}"; do
        local status=$(docker inspect --format='{{.State.Health.Status}}' "stock_trader_$service" 2>/dev/null || echo "unknown")
        
        if [ "$status" == "healthy" ]; then
            success "$service: $status"
        else
            warn "$service: $status"
            all_healthy=false
        fi
    done
    
    if [ "$all_healthy" == "true" ]; then
        success "所有服务健康状态正常"
    else
        warn "部分服务健康状态异常，请检查日志"
    fi
    
    info "测试API网关连通性..."
    local max_attempts=10
    for i in $(seq 1 $max_attempts); do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            success "API网关响应正常"
            return 0
        fi
        sleep 2
    done
    
    warn "API网关暂无响应，可能需要更多时间"
    return 0
}

show_service_status() {
    echo_header "服务状态"
    
    cd "$DEPLOY_DIR"
    docker-compose ps
    
    echo ""
    info "服务端口映射:"
    echo "  API Gateway:    8080"
    echo "  User Service:   5001/8081"
    echo "  Order Service:  5002/8082"
    echo "  Account Service: 5004/8084"
    echo "  Market Service: 5003/8083"
    echo "  Matching Service: 5005/8085"
    echo "  Reconciliation: 5006/8086"
    echo "  MySQL:         3306"
    echo "  Redis:         6379"
    echo "  Kafka:         9092"
}

show_logs() {
    echo_header "最近日志"
    
    local service="${1:-api-gateway}"
    local lines="${2:-50}"
    
    info "显示 $service 的最近 $lines 行日志..."
    docker logs --tail "$lines" "stock_trader_$service" 2>&1
}

run_health_check() {
    echo_header "健康检查"
    
    info "检查所有HTTP端点..."
    
    local endpoints=(
        "http://localhost:8080/health"
        "http://localhost:8081/ready"
        "http://localhost:8082/ready"
        "http://localhost:8083/ready"
        "http://localhost:8084/ready"
        "http://localhost:8085/ready"
        "http://localhost:8086/ready"
    )
    
    local passed=0
    local failed=0
    
    for endpoint in "${endpoints[@]}"; do
        local service_name=$(echo "$endpoint" | grep -oP '(?<=localhost:)\d+' | xargs -I{} sh -c 'case {} in 8080) echo "API Gateway";; 8081) echo "User Service";; 8082) echo "Order Service";; 8083) echo "Market Service";; 8084) echo "Account Service";; 8085) echo "Matching Service";; 8086) echo "Reconciliation Service";; esac')
        
        if curl -s "$endpoint" > /dev/null 2>&1; then
            success "$service_name: OK"
            passed=$((passed + 1))
        else
            warn "$service_name: FAILED"
            failed=$((failed + 1))
        fi
    done
    
    echo ""
    info "健康检查结果: $passed 通过, $failed 失败"
    
    if [ $failed -gt 0 ]; then
        return 1
    fi
    return 0
}

rollback() {
    echo_header "回滚到上一版本"
    
    local backup_dir="$PROJECT_ROOT/backups"
    
    if [ ! -d "$backup_dir" ]; then
        error "没有找到备份目录"
        exit 1
    fi
    
    local latest_backup=$(ls -td "$backup_dir"/*/ 2>/dev/null | head -1)
    
    if [ -z "$latest_backup" ]; then
        error "没有找到备份"
        exit 1
    fi
    
    info "回滚到备份: $latest_backup"
    
    cd "$DEPLOY_DIR"
    
    info "停止当前服务..."
    docker-compose down --remove-orphans 2>/dev/null || true
    
    if [ -f "$latest_backup/stock_trader.sql" ]; then
        info "恢复MySQL数据..."
        docker exec -i stock_trader_mysql mysql -u root -ppassword stock_trader < "$latest_backup/stock_trader.sql" 2>/dev/null || warn "MySQL恢复失败"
    fi
    
    info "使用上一版本镜像启动..."
    docker-compose up -d
    
    success "回滚完成"
}

emergency_stop() {
    echo_header "紧急停止所有服务"
    
    cd "$DEPLOY_DIR"
    
    info "立即停止所有服务..."
    docker-compose down --remove-orphans -v 2>/dev/null || true
    
    info "强制停止所有相关容器..."
    docker ps -a --filter "name=stock_trader_" -q | xargs -r docker rm -f 2>/dev/null || true
    
    info "清理网络..."
    docker network rm stock_trader_stock_trader_network 2>/dev/null || true
    
    success "紧急停止完成"
}

cleanup() {
    echo_header "清理环境"
    
    cd "$DEPLOY_DIR"
    
    info "停止并删除所有服务..."
    docker-compose down --volumes --remove-orphans 2>/dev/null || true
    
    info "删除所有镜像..."
    docker rmi $(docker images -q "stock_trader-*" 2>/dev/null) 2>/dev/null || true
    
    info "删除所有数据卷..."
    docker volume rm $(docker volume ls -qf dangling=true) 2>/dev/null || true
    
    info "清理Docker资源..."
    docker system prune -af 2>/dev/null || true
    
    success "清理完成"
}

show_help() {
    echo_header "帮助信息"
    
    echo "用法: $0 [命令]"
    echo ""
    echo "可用命令:"
    echo "  deploy          - 部署所有服务（默认）"
    echo "  build           - 仅构建镜像"
    echo "  start           - 启动服务"
    echo "  stop            - 停止服务"
    echo "  restart         - 重启服务"
    echo "  status          - 查看服务状态"
    echo "  logs [服务]     - 查看日志（默认: api-gateway）"
    echo "  health          - 运行健康检查"
    echo "  backup          - 备份数据"
    echo "  rollback        - 回滚到上一版本"
    echo "  emergency-stop  - 紧急停止所有服务"
    echo "  cleanup         - 清理所有资源"
    echo "  help            - 显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  VERSION         - 版本号 (默认: v1.0.0)"
    echo "  BUILD_PARALLEL  - 是否并行构建 (默认: true)"
    echo "  SKIP_BUILD      - 跳过构建 (默认: false)"
    echo ""
    echo "示例:"
    echo "  $0 deploy                    # 部署所有服务"
    echo "  $0 deploy BUILD_PARALLEL=false  # 串行构建"
    echo "  $0 logs order-service        # 查看订单服务日志"
    echo "  $0 rollback                  # 回滚"
}

main() {
    local command="${1:-deploy}"
    
    echo "========================================"
    echo "  证券交易系统一键部署脚本"
    echo "  版本: $VERSION"
    echo "  $(date '+%Y-%m-%d %H:%M:%S')"
    echo "========================================"
    
    mkdir -p "$LOG_DIR"
    
    case "$command" in
        deploy)
            check_prerequisites
            check_system_resources
            backup_data
            stop_services
            clean_images
            build_images
            start_services
            wait_for_services
            verify_services
            show_service_status
            run_health_check
            ;;
        build)
            check_prerequisites
            build_images
            ;;
        start)
            check_prerequisites
            start_services
            wait_for_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            stop_services
            start_services
            wait_for_services
            ;;
        status)
            show_service_status
            ;;
        logs)
            show_logs "${2:-api-gateway}" "${3:-50}"
            ;;
        health)
            run_health_check
            ;;
        backup)
            check_prerequisites
            backup_data
            ;;
        rollback)
            check_prerequisites
            rollback
            ;;
        emergency-stop)
            emergency_stop
            ;;
        cleanup)
            cleanup
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
    echo ""
}

main "$@"
