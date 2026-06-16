#!/bin/bash
# ──────────────────────────────────────────────
# start.sh — Build and run fastcopy
# ──────────────────────────────────────────────
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

print_header() {
    echo -e "${CYAN}"
    echo "  ⚡ fastcopy — Ultra-Fast Parallel File Copier"
    echo "  ─────────────────────────────────────────────"
    echo -e "${NC}"
}

usage() {
    print_header
    echo -e "${BOLD}Usage:${NC} $0 <command> [options]"
    echo ""
    echo -e "${BOLD}Commands:${NC}"
    echo "  build         Build the CLI binary"
    echo "  build-gui     Build the GUI binary (requires X11/OpenGL dev libs)"
    echo "  build-all     Build both CLI and GUI"
    echo "  run           Build and run CLI with the given arguments"
    echo "  run-gui       Build and run the GUI"
    echo "  test          Run integration tests"
    echo "  deps          Install system dependencies for GUI (requires sudo)"
    echo "  clean         Remove built binaries"
    echo "  help          Show this help message"
    echo ""
    echo -e "${BOLD}Examples:${NC}"
    echo "  $0 build"
    echo "  $0 run /data/source /backup/dest"
    echo "  $0 run -w 32 --checksum /src /dst"
    echo "  $0 run-gui"
    echo "  $0 test"
    echo ""
}

build_cli() {
    echo -e "${YELLOW}▸ Building fastcopy CLI...${NC}"
    go build -o "$SCRIPT_DIR/fastcopy" ./cmd/fastcopy/
    echo -e "${GREEN}✔ Built: ${BOLD}$SCRIPT_DIR/fastcopy${NC}"
}

build_gui() {
    echo -e "${YELLOW}▸ Checking GUI dependencies...${NC}"

    # Check for X11 dev headers
    if ! pkg-config --exists x11 2>/dev/null; then
        echo -e "${RED}✘ Missing X11 development libraries.${NC}"
        echo -e "  Run: ${BOLD}$0 deps${NC} to install them."
        exit 1
    fi

    echo -e "${YELLOW}▸ Building fastcopy-gui (this may take a while on first run)...${NC}"
    go build -o "$SCRIPT_DIR/fastcopy-gui" ./cmd/fastcopy-gui/
    echo -e "${GREEN}✔ Built: ${BOLD}$SCRIPT_DIR/fastcopy-gui${NC}"
}

install_deps() {
    echo -e "${YELLOW}▸ Installing system dependencies for Fyne GUI...${NC}"

    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq
        sudo apt-get install -y \
            libx11-dev libxcursor-dev libxrandr-dev \
            libxinerama-dev libxi-dev libglx-dev \
            libgl1-mesa-dev libxxf86vm-dev \
            pkg-config
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y \
            libX11-devel libXcursor-devel libXrandr-devel \
            libXinerama-devel libXi-devel mesa-libGL-devel \
            pkg-config
    elif command -v pacman &>/dev/null; then
        sudo pacman -S --noconfirm \
            libx11 libxcursor libxrandr \
            libxinerama libxi mesa \
            pkg-config
    else
        echo -e "${RED}✘ Unsupported package manager. Install X11/OpenGL dev libs manually.${NC}"
        exit 1
    fi

    echo -e "${GREEN}✔ Dependencies installed.${NC}"
}

run_tests() {
    echo -e "${YELLOW}▸ Building CLI...${NC}"
    build_cli

    echo -e "${YELLOW}▸ Running integration tests...${NC}"
    echo ""

    TMPDIR=$(mktemp -d "$SCRIPT_DIR/.test_XXXXXX")
    trap "rm -rf $TMPDIR" EXIT

    mkdir -p "$TMPDIR/src/subdir"

    # Create test files
    for i in $(seq 1 100); do
        echo "test data $i" > "$TMPDIR/src/file_$i.txt"
    done
    dd if=/dev/urandom of="$TMPDIR/src/large.bin" bs=1M count=10 2>/dev/null
    for i in $(seq 1 20); do
        echo "nested $i" > "$TMPDIR/src/subdir/nested_$i.txt"
    done
    ln -s file_1.txt "$TMPDIR/src/link.txt"

    TOTAL_FILES=$(find "$TMPDIR/src" -not -type d | wc -l)
    TOTAL_SIZE=$(du -sh "$TMPDIR/src" | cut -f1)
    echo -e "  Test dataset: ${BOLD}$TOTAL_FILES files, $TOTAL_SIZE${NC}"

    # Test 1: Initial copy
    echo -ne "  Test 1: Initial copy.............. "
    "$SCRIPT_DIR/fastcopy" --quiet "$TMPDIR/src" "$TMPDIR/dst1"
    if diff -rq "$TMPDIR/src" "$TMPDIR/dst1" >/dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        exit 1
    fi

    # Test 2: Incremental (no changes)
    echo -ne "  Test 2: Incremental skip.......... "
    OUTPUT=$("$SCRIPT_DIR/fastcopy" "$TMPDIR/src" "$TMPDIR/dst1" 2>&1)
    if echo "$OUTPUT" | grep -q "Skipped:.*files"; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        exit 1
    fi

    # Test 3: Force recopy
    echo -ne "  Test 3: Force recopy.............. "
    "$SCRIPT_DIR/fastcopy" --quiet --force "$TMPDIR/src" "$TMPDIR/dst2"
    if diff -rq "$TMPDIR/src" "$TMPDIR/dst2" >/dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        exit 1
    fi

    # Test 4: Checksum
    echo -ne "  Test 4: SHA256 checksum........... "
    OUTPUT=$("$SCRIPT_DIR/fastcopy" --quiet --checksum --force "$TMPDIR/src" "$TMPDIR/dst3" 2>&1)
    if echo "$OUTPUT" | grep -qE "^  [a-f0-9]{64}  "; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        exit 1
    fi

    # Test 5: Dry run
    echo -ne "  Test 5: Dry run................... "
    OUTPUT=$("$SCRIPT_DIR/fastcopy" --dry-run --force "$TMPDIR/src" "$TMPDIR/dst4" 2>&1)
    if echo "$OUTPUT" | grep -q "DRY RUN" && [ ! -d "$TMPDIR/dst4" ]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        exit 1
    fi

    # Test 6: Remove source
    echo -ne "  Test 6: Remove source............. "
    # Re-create some files in a temporary source just for this test
    mkdir -p "$TMPDIR/src_move/subdir"
    echo "move me" > "$TMPDIR/src_move/move.txt"
    OUTPUT=$("$SCRIPT_DIR/fastcopy" --quiet --remove-source "$TMPDIR/src_move" "$TMPDIR/dst5" 2>&1)
    if [ ! -f "$TMPDIR/src_move/move.txt" ] && [ -f "$TMPDIR/dst5/move.txt" ] && [ ! -d "$TMPDIR/src_move/subdir" ] && [ ! -d "$TMPDIR/src_move" ]; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
        exit 1
    fi

    echo ""
    echo -e "${GREEN}${BOLD}All tests passed!${NC}"
}

# ── Main ──
case "${1:-help}" in
    build)
        print_header
        build_cli
        ;;
    build-gui)
        print_header
        build_gui
        ;;
    build-all)
        print_header
        build_cli
        build_gui
        ;;
    run)
        build_cli
        shift
        exec "$SCRIPT_DIR/fastcopy" "$@"
        ;;
    run-gui)
        build_gui
        exec env LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8 "$SCRIPT_DIR/fastcopy-gui"
        ;;
    test)
        print_header
        run_tests
        ;;
    deps)
        print_header
        install_deps
        ;;
    clean)
        print_header
        rm -f "$SCRIPT_DIR/fastcopy" "$SCRIPT_DIR/fastcopy-gui"
        echo -e "${GREEN}✔ Cleaned.${NC}"
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        echo ""
        usage
        exit 1
        ;;
esac
