#!/bin/bash

# ========== 参数解析 ==========
PROXY_ARG=""
while getopts ":x:" opt; do
  case $opt in
    x)
      # 注意去除前面的等号（比如 -x=http://... 传入时可能带=）
      proxy_val="${OPTARG#*=}"
      if [[ -z "$proxy_val" ]]; then
        proxy_val="$OPTARG"
      fi
      PROXY_ARG="--proxy $proxy_val"
      echo "✅ 使用代理：$proxy_val"
      ;;
    *)
      echo "❌ 未知参数"
      exit 1
      ;;
  esac
done

# ========== 路径定义 ==========
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
NODELIST_FILE="$SCRIPT_DIR/../source/nodelist.txt"
RESULT_FILE="$SCRIPT_DIR/result.log"

if [ ! -f "$NODELIST_FILE" ]; then
  echo "❌ 节点列表文件不存在：$NODELIST_FILE"
  exit 1
fi

# ========== 时间比较函数 ==========
month=2592000
timestamp=$(date +%s)
lastMonth=$((timestamp - month))

function isUpdated() {
  # macOS 可能没有 gdate，优先用 date -d ，失败用 gdate -d
  t1=$(date -d "$1" +%s 2>/dev/null || gdate -d "$1" +%s 2>/dev/null)
  if [ -z "$t1" ]; then return 1; fi
  [ "$t1" -gt "$lastMonth" ]
}

# ========== 主逻辑 ==========
rm -f "$RESULT_FILE"
i=0

while IFS= read -r url || [ -n "$url" ]; do
  [[ -z "$url" || "$url" != https://raw.githubusercontent.com* ]] && continue

  i=$((i + 1))
  code=$(curl $PROXY_ARG -o /dev/null -k -s -w "%{http_code}" "$url")
  if [ "$code" -ne 404 ]; then
    repo_path=$(echo "$url" | cut -d '/' -f 4,5)
    repo_api="https://api.github.com/repos/$repo_path"

    response=$(curl $PROXY_ARG -s -H "User-Agent: alive-script" "$repo_api")
    updated_at=$(echo "$response" | jq -r '.updated_at // empty')

    if [ -n "$updated_at" ]; then
      updated_at=${updated_at:0:19}
      updated_at=${updated_at/T/' '}
      if isUpdated "$updated_at"; then
        echo "$url" >> "$RESULT_FILE"
        echo "✅ $url 最近更新：$updated_at"
      else
        echo "⏱ $url 超过一个月未更新：$updated_at"
      fi
    else
      echo "⚠️ 无法获取更新时间：$repo_api"
      echo "🔍 响应内容（截断）："
      echo "$response" | head -n 5
    fi
  fi

  if ((i % 8 == 0)); then
    echo "⏳ 暂停 120 秒，防止限流..."
    sleep 120
  fi
done < "$NODELIST_FILE"


echo "✅ 扫描完成，结果已写入：$RESULT_FILE"