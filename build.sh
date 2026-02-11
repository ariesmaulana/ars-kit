#!/bin/bash

# Build script for expend application
# Supports both macOS and Linux (Debian/Ubuntu)

set -e

# Configuration
APP_NAME="expend"
MAIN_PATH="src/main.go"
BUILD_DIR="bin"
VERSION="${VERSION:-dev}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
Usage: ./build.sh [OPTIONS]

Build script for expend application

OPTIONS:
    -h, --help              Show this help message
    -c, --clean             Clean build artifacts before building
    -t, --test              Run tests before building
    -l, --linux             Build for Linux (Debian/Ubuntu) amd64
    -m, --mac               Build for macOS amd64
    -a, --all               Build for all platforms
    -g, --generate          Run go generate before building
    -v, --version VERSION   Set version (default: dev)

EXAMPLES:
    ./build.sh                      # Build for current OS
    ./build.sh --linux              # Build for Linux
    ./build.sh --all --test         # Build all platforms with tests
    ./build.sh --clean --linux      # Clean and build for Linux

EOF
}

clean_build() {
    log_info "Cleaning build artifacts..."
    rm -rf "${BUILD_DIR}"
    go clean
    log_info "Clean complete"
}

run_tests() {
    log_info "Running tests..."
    if go test ./... -count=1; then
        log_info "Tests passed ✓"
    else
        log_error "Tests failed ✗"
        exit 1
    fi
}

generate_code() {
    log_info "Running go generate..."
    go generate ./...
    log_info "Code generation complete"
}

build_for_os() {
    local os=$1
    local arch=$2
    local output_name="${BUILD_DIR}/${APP_NAME}-${os}-${arch}"
    
    log_info "Building for ${os}/${arch}..."
    
    mkdir -p "${BUILD_DIR}"
    
    GOOS=${os} GOARCH=${arch} go build \
        -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
        -o "${output_name}" \
        "${MAIN_PATH}"
    
    if [ $? -eq 0 ]; then
        log_info "Build successful: ${output_name}"
        
        # Make executable
        chmod +x "${output_name}"
        
        # Show file info
        if command -v file &> /dev/null; then
            file "${output_name}"
        fi
        
        # Show size
        if [ -f "${output_name}" ]; then
            size=$(du -h "${output_name}" | cut -f1)
            log_info "Binary size: ${size}"
        fi
    else
        log_error "Build failed for ${os}/${arch}"
        exit 1
    fi
}

# Parse arguments
CLEAN=false
TEST=false
BUILD_LINUX=false
BUILD_MAC=false
BUILD_ALL=false
GENERATE=false

if [ $# -eq 0 ]; then
    # Default: build for current OS
    CURRENT_OS=$(go env GOOS)
    CURRENT_ARCH=$(go env GOARCH)
    build_for_os "${CURRENT_OS}" "${CURRENT_ARCH}"
    exit 0
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -t|--test)
            TEST=true
            shift
            ;;
        -l|--linux)
            BUILD_LINUX=true
            shift
            ;;
        -m|--mac)
            BUILD_MAC=true
            shift
            ;;
        -a|--all)
            BUILD_ALL=true
            shift
            ;;
        -g|--generate)
            GENERATE=true
            shift
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Execute based on flags
if [ "$CLEAN" = true ]; then
    clean_build
fi

if [ "$GENERATE" = true ]; then
    generate_code
fi

if [ "$TEST" = true ]; then
    run_tests
fi

# Build targets
if [ "$BUILD_ALL" = true ]; then
    log_info "Building for all platforms..."
    build_for_os "linux" "amd64"
    build_for_os "darwin" "amd64"
    build_for_os "darwin" "arm64"
elif [ "$BUILD_LINUX" = true ]; then
    build_for_os "linux" "amd64"
elif [ "$BUILD_MAC" = true ]; then
    build_for_os "darwin" "amd64"
else
    # Build for current OS if no specific target specified
    CURRENT_OS=$(go env GOOS)
    CURRENT_ARCH=$(go env GOARCH)
    build_for_os "${CURRENT_OS}" "${CURRENT_ARCH}"
fi

log_info "Build process complete!"
