#!/bin/bash

# Vue Chat App 开发脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 帮助信息
show_help() {
    echo "Vue Chat App 开发脚本"
    echo ""
    echo "用法: ./dev.sh [命令]"
    echo ""
    echo "命令:"
    echo "  install     安装依赖"
    echo "  dev         启动开发服务器 (需要后端运行在 localhost:8080)"
    echo "  build       构建生产版本"
    echo "  preview     预览生产构建"
    echo "  lint        代码检查"
    echo "  clean       清理构建产物"
    echo "  help        显示帮助信息"
    echo ""
}

# 检查 Node.js
check_node() {
    if ! command -v node &> /dev/null; then
        echo -e "${RED}错误: 未找到 Node.js，请先安装 Node.js${NC}"
        exit 1
    fi
    echo -e "${GREEN}Node.js 版本: $(node -v)${NC}"
}

# 检查 npm
check_npm() {
    if ! command -v npm &> /dev/null; then
        echo -e "${RED}错误: 未找到 npm${NC}"
        exit 1
    fi
    echo -e "${GREEN}npm 版本: $(npm -v)${NC}"
}

# 安装依赖
install_deps() {
    echo -e "${YELLOW}安装依赖...${NC}"
    check_node
    check_npm
    npm install
    echo -e "${GREEN}依赖安装完成${NC}"
}

# 启动开发服务器
start_dev() {
    echo -e "${YELLOW}启动开发服务器...${NC}"
    echo -e "${YELLOW}注意: 确保后端服务运行在 localhost:8080${NC}"
    echo ""
    npm run dev
}

# 构建生产版本
build_prod() {
    echo -e "${YELLOW}构建生产版本...${NC}"
    npm run build
    echo -e "${GREEN}构建完成，产物在 dist/ 目录${NC}"
}

# 预览构建
preview_build() {
    echo -e "${YELLOW}预览生产构建...${NC}"
    npm run preview
}

# 代码检查
run_lint() {
    echo -e "${YELLOW}运行代码检查...${NC}"
    npm run lint 2>/dev/null || echo -e "${YELLOW}lint 命令未配置${NC}"
}

# 清理
clean() {
    echo -e "${YELLOW}清理构建产物...${NC}"
    rm -rf dist node_modules/.vite
    echo -e "${GREEN}清理完成${NC}"
}

# 主函数
main() {
    case "${1:-help}" in
        install)
            install_deps
            ;;
        dev)
            start_dev
            ;;
        build)
            build_prod
            ;;
        preview)
            preview_build
            ;;
        lint)
            run_lint
            ;;
        clean)
            clean
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo -e "${RED}未知命令: $1${NC}"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

main "$@"
