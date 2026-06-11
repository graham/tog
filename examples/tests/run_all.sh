#!/bin/bash
# Run all inline tests
# Usage: ./inline_tests/run_all.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(dirname "$SCRIPT_DIR")"
cd "$EXAMPLES_DIR"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Running inline tests...${NC}"
echo ""

TOTAL=0
PASSED=0

for test_file in "$SCRIPT_DIR"/*.txt; do
    test_name=$(basename "$test_file")
    echo -e "${BLUE}=== $test_name ===${NC}"

    TOTAL=$((TOTAL + 1))

    # Run the test and capture output
    if go run main.go inlinetest -f "$test_file" 2>&1 | grep -E '(PASS|FAIL|->)'; then
        # Check if there were any failures
        if go run main.go inlinetest -f "$test_file" 2>&1 | grep -q 'FAIL'; then
            echo -e "${RED}FAILED: $test_name${NC}"
        else
            echo -e "${GREEN}PASSED: $test_name${NC}"
            PASSED=$((PASSED + 1))
        fi
    fi
    echo ""
done

echo -e "${BLUE}==============================${NC}"
echo -e "Results: ${PASSED}/${TOTAL} test files passed"

if [ "$PASSED" -eq "$TOTAL" ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed${NC}"
    exit 1
fi
