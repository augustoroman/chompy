#!/bin/bash -ex
# This is a simple command-line utility to manually grant rewards to people.

AUTH=${CHOMPY_AUTH:?"Must provide CHOMPY_AUTH secret auth token"}
URL=${CHOMPY_HOST:?"Must provide CHOMPY_HOST for chompy's hostname, e.g. chompy.example.com"}
read -p "Email: " EMAIL
read -p "Reason: " REASON
TYPE="manual"

curl -X PUT -F auth=$AUTH -F email="$EMAIL" -F type="$TYPE" -F desc="$REASON" http://$CHOMPY_HOST/r
