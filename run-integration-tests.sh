#!/bin/bash
# Integration tests runner script
# Usage: ./run-integration-tests.sh [--clean] [--no-cleanup]
#
# Flags:
#   --clean       Remove containers and volumes before running tests
#   --no-cleanup  Keep containers and volumes after tests finish

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CLEAN=false
CLEANUP=true
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.integration.yml"

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

echo -e "${BLUE}=== Integration Tests Runner ===${NC}"

if [ "$CLEAN" = true ]; then
    echo -e "${BLUE}Cleaning up existing containers and volumes...${NC}"
    docker compose -f "$COMPOSE_FILE" down -v || true
fi

# Start services
echo -e "${BLUE}Starting services...${NC}"
docker compose -f "$COMPOSE_FILE" up -d db redis

# Wait for services to be ready
echo -e "${BLUE}Waiting for services to be ready...${NC}"
sleep 5

# Run integration tests
echo -e "${BLUE}Running integration tests...${NC}"
docker compose -f "$COMPOSE_FILE" build integration-tests
docker compose -f "$COMPOSE_FILE" run --rm integration-tests

TEST_EXIT_CODE=$?

if [ "$CLEANUP" = true ]; then
    echo -e "${BLUE}Cleaning up...${NC}"
    docker compose -f "$COMPOSE_FILE" down -v
fi

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ Integration tests passed!${NC}"
else
    echo -e "${RED}✗ Integration tests failed!${NC}"
fi

exit $TEST_EXIT_CODE
