#!/bin/bash
# Update coverage badge in README.md with proper color thresholds
# Thresholds: red < 60%, yellow 60-80%, green >= 80%

set -e

# Extract coverage percentage
coverage=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')

echo "Coverage: ${coverage}%"

# Determine badge color based on thresholds
if awk "BEGIN {exit !($coverage >= 80)}"; then
    color="green"
elif awk "BEGIN {exit !($coverage >= 60)}"; then
    color="yellow"
else
    color="red"
fi

echo "Badge color: $color"

# Generate badge URL
badge_url="https://img.shields.io/badge/Coverage-${coverage}%25-${color}"

echo "Badge URL: $badge_url"

# Update README.md
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s|!\[Coverage\](https://img.shields.io/badge/Coverage-[0-9.]*%25-[a-z]*)|![Coverage](${badge_url})|" README.md
else
    # Linux
    sed -i "s|!\[Coverage\](https://img.shields.io/badge/Coverage-[0-9.]*%25-[a-z]*)|![Coverage](${badge_url})|" README.md
fi

echo "✓ README.md updated successfully"
