#!/bin/bash
# 既存のdocker-compose.ymlをそのまま使う最もシンプルな方法

set -euo pipefail

echo "🚀 Starting docker-compose with dynamic ports..."

# ポート割り当て + 環境変数を一気にセット
eval "$(go-portalloc create --ports 10 --shell)"

echo "✅ Allocated resources:"
echo "   COMPOSE_PROJECT_NAME: $COMPOSE_PROJECT_NAME"
echo "   Isolation ID: $ISOLATION_ID"
echo "   Port range: $PORT_BASE-$((PORT_BASE + PORT_COUNT - 1))"
echo ""

# クリーンアップを設定
trap "docker-compose down && go-portalloc cleanup --id $ISOLATION_ID" EXIT

# 既存のdocker-compose.ymlをそのまま起動
# COMPOSE_PROJECT_NAME が自動的に設定されているので、コンテナ名が衝突しない
docker-compose up -d

echo ""
echo "✅ Services started with project: $COMPOSE_PROJECT_NAME"
echo "   Run 'docker-compose ps' to see running services"
echo ""

# サービスが起動するまで待機
sleep 5

# テストやその他の処理を実行
echo "🧪 Running tests..."
# your test command here
# pytest tests/
# go test ./...

echo ""
echo "✅ All done! Cleanup will run automatically on exit."
