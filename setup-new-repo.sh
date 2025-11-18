#!/bin/bash
# Helper script to set up a new git repository

echo "🚀 Setting up new repository"
echo ""
echo "This script will:"
echo "1. Remove the current remote"
echo "2. Add your new remote"
echo "3. Push to the new repository"
echo ""

read -p "Enter your new repository URL (e.g., https://github.com/username/repo.git): " REPO_URL

if [ -z "$REPO_URL" ]; then
    echo "❌ Error: Repository URL is required"
    exit 1
fi

echo ""
echo "Removing old remote..."
git remote remove origin

echo "Adding new remote: $REPO_URL"
git remote add origin "$REPO_URL"

echo ""
echo "Pushing to new repository..."
git push -u origin main

echo ""
echo "✅ Done! Your repository is now set up at: $REPO_URL"

