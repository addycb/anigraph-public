#!/bin/bash
# Build and run the Go scraper

echo "Building scraper..."
go build -o scraper scrape.go

if [ $? -eq 0 ]; then
    echo "Build successful! Starting scraper..."
    ./scraper
else
    echo "Build failed!"
    exit 1
fi
