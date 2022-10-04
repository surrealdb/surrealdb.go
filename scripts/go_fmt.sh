#!/bin/sh

if [ "$(gofmt -s -l . | wc -l)" -gt 0 ]; then
    gofmt -d .
    exit 1
fi
exit 0
