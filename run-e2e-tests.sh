#!/bin/bash
# E2E tests runner script
# Usage: ./run-e2e-tests.sh [--clean] [--no-cleanup]

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CLEAN=false
CLEANUP=true
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.e2e.yml"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --clean)
            CLEAN=true
            shift
            ;;
        --no-cleanup)
            CLEANUP=false
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== E2E Tests Runner ===${NC}"

if [ "$CLEAN" = true ]; then
    echo -e "${BLUE}Cleaning up existing containers and volumes...${NC}"
    docker compose -f "$COMPOSE_FILE" down -v || true
fi

# Start services and run tests
echo -e "${BLUE}Starting services and running E2E tests...${NC}"
docker compose -f "$COMPOSE_FILE" up --build --abort-on-container-exit --exit-code-from e2e-tests

TEST_EXIT_CODE=$?

if [ "$CLEANUP" = true ]; then
    echo -e "${BLUE}Cleaning up...${NC}"
    docker compose -f "$COMPOSE_FILE" down -v
fi

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ E2E tests passed!${NC}"
else
    echo -e "${RED}✗ E2E tests failed!${NC}"
fi

exit $TEST_EXIT_CODE
